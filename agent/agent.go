package agent

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	ossignal "os/signal"
	"syscall"
	"time"

	"github.com/bienkma/luks-vault/config"
	"github.com/bienkma/luks-vault/module"
	"github.com/sevlyar/go-daemon"
)

var (
	signal = flag.String("s", "", `Send signal to the daemon:
	quit - graceful shutdown
	stop - fast shutdown
	reload - reloading the configuration file`)
	foreground = flag.Bool("foreground", false, "Run in the foreground without daemon fork (for systemd)")
	stop         = make(chan struct{})
	done         = make(chan struct{})
	reloadAgent  *Instances
	currentKeyName = "key"
	newKeyName     = "newKey"
)

type Instances struct {
	Vault   module.VaultAgent
	Luks    module.LUKSOperation
	runtime *runtime
}

func New() *Instances {
	return &Instances{}
}

func (a *Instances) Start(ctx context.Context) {
	cfg := config.New()

	flag.Parse()
	a.runtime = newRuntime(cfg)
	a.syncLUKSConfig(cfg)
	reloadAgent = a

	if *foreground {
		a.runForeground(ctx)
		return
	}

	daemon.AddCommand(daemon.StringFlag(signal, "quit"), syscall.SIGQUIT, termHandle)
	daemon.AddCommand(daemon.StringFlag(signal, "reload"), syscall.SIGHUP, reloadHandler)

	cntxt := &daemon.Context{
		PidFileName: cfg.Agent.PIDFile,
		PidFilePerm: 0600,
		LogFileName: cfg.Agent.LogFile,
		LogFilePerm: 0640,
		Umask:       027,
	}

	if len(daemon.ActiveFlags()) > 0 {
		d, err := cntxt.Search()
		if err != nil {
			log.Fatalf("Unable send signal to the daemon: %s", err.Error())
		}
		daemon.SendCommands(d)
		return
	}
	d, err := cntxt.Reborn()
	if err != nil {
		log.Fatalln(err)
	}
	if d != nil {
		return
	}
	defer cntxt.Release()
	log.Println("- - - - - - - - - - - - - - -")
	log.Println("luks-vault daemon started")

	go func() {
		if err := a.runWorker(ctx); err != nil {
			log.Printf("worker stopped with error: %v", err)
		}
	}()

	err = daemon.ServeSignals()
	if err != nil {
		log.Printf("error: %s", err.Error())
	}

	log.Println("luks-vault daemon terminated")
}

func (a *Instances) runForeground(ctx context.Context) {
	cfg := a.runtime.Config()
	if err := setupLogging(cfg.Agent.LogFile); err != nil {
		fmt.Fprintf(os.Stderr, "unable to open log file %s: %v\n", cfg.Agent.LogFile, err)
		os.Exit(1)
	}

	log.Println("luks-vault started in foreground mode")
	if cfg.LUKS.CryptsetupUseSudo {
		log.Println("cryptsetup will run via sudo")
	}

	sigCh := make(chan os.Signal, 1)
	ossignal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGHUP)

	workerErr := make(chan error, 1)
	go func() {
		workerErr <- a.runWorker(ctx)
	}()

	for {
		select {
		case sig := <-sigCh:
			if sig == syscall.SIGHUP {
				if err := a.runtime.Reload(); err != nil {
					log.Printf("configuration reload failed: %v", err)
				} else {
					a.syncLUKSConfig(a.runtime.Config())
				}
				continue
			}

			log.Printf("received signal %v, shutting down", sig)
			stop <- struct{}{}
			<-done
			log.Println("luks-vault terminated")
			return
		case err := <-workerErr:
			if err != nil {
				log.Printf("worker stopped with error: %v", err)
				os.Exit(1)
			}
			log.Println("luks-vault worker exited")
			return
		}
	}
}

func (a *Instances) runWorker(ctx context.Context) error {
	defer func() {
		done <- struct{}{}
	}()

	if err := a.initVault(); err != nil {
		return err
	}

	rotationBackoff := time.Duration(0)

	for {
		cfg := a.runtime.Config()
		wait := cfg.Agent.PollInterval
		if rotationBackoff > 0 {
			wait = rotationBackoff
		}

		if stopped := a.waitOrStop(ctx, wait); stopped {
			return nil
		}

		select {
		case <-a.runtime.reload:
			if err := a.initVault(); err != nil {
				log.Printf("vault re-initialization failed after reload: %v", err)
			}
			a.syncLUKSConfig(a.runtime.Config())
		default:
		}

		vaultData, err := a.Vault.GetSecret(ctx, cfg.Vault.KV2Mount, cfg.Vault.SecretPath)
		if err != nil {
			log.Printf("unable to get vault secret %s/%s: %v", cfg.Vault.KV2Mount, cfg.Vault.SecretPath, err)
			continue
		}

		expired, err := isTTLExpired(vaultData.Created, vaultData.TTL, time.Now())
		if err != nil {
			log.Printf("invalid TTL metadata in vault secret: %v", err)
			continue
		}
		if !expired {
			rotationBackoff = 0
			continue
		}

		if !cfg.LUKS.Enabled {
			log.Printf("vault secret TTL expired but luks.enabled is false, skipping rotation")
			continue
		}

		if err := a.rotatePassphrase(ctx, cfg, vaultData); err != nil {
			log.Printf("passphrase rotation failed: %v", err)
			rotationBackoff = nextRotationBackoff(rotationBackoff)
			log.Printf("next rotation retry in %s", rotationBackoff)
			continue
		}

		rotationBackoff = 0
	}
}

func (a *Instances) initVault() error {
	cfg := a.runtime.Config()
	if err := a.Vault.Configure(cfg.Vault); err != nil {
		return fmt.Errorf("configure vault client: %w", err)
	}
	return nil
}

func (a *Instances) syncLUKSConfig(cfg *config.Configuration) {
	a.Luks.UseSudo = cfg.LUKS.CryptsetupUseSudo
}

func (a *Instances) waitOrStop(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-stop:
		return true
	case <-ctx.Done():
		return true
	case <-timer.C:
		return false
	}
}

func termHandle(sig os.Signal) error {
	log.Println("terminating...")
	stop <- struct{}{}
	if sig == syscall.SIGQUIT {
		<-done
	}
	return daemon.ErrStop
}

func reloadHandler(sig os.Signal) error {
	if reloadAgent == nil {
		return fmt.Errorf("agent is not initialized")
	}
	if err := reloadAgent.runtime.Reload(); err != nil {
		return err
	}
	reloadAgent.syncLUKSConfig(reloadAgent.runtime.Config())
	return nil
}
