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
	vault "github.com/hashicorp/vault/api"
	"github.com/sethvargo/go-password/password"
	"github.com/sevlyar/go-daemon"
)

const pollInterval = 10 * time.Second

var (
	signal = flag.String("s", "", `Send signal to the daemon:
	quit - graceful shutdown
	stop - fast shutdown
	reload - reloading the configuration file`)
	foreground = flag.Bool("foreground", false, "Run in the foreground without daemon fork (for systemd)")
	stop           = make(chan struct{})
	done           = make(chan struct{})
	currentKeyName = "key"
	newKeyName     = "newKey"
)

type Instances struct {
	Vault module.VaultAgent
	Luks  module.LUKSOperation
}

func New() *Instances {
	return &Instances{}
}

func (a *Instances) Start(ctx context.Context) {
	cfg := config.New()

	flag.Parse()
	a.Luks.UseSudo = cfg.CryptsetupUseSudo

	if *foreground {
		a.runForeground(ctx, cfg)
		return
	}

	daemon.AddCommand(daemon.StringFlag(signal, "quit"), syscall.SIGQUIT, termHandle)
	daemon.AddCommand(daemon.StringFlag(signal, "reload"), syscall.SIGHUP, reloadHandler)

	cntxt := &daemon.Context{
		PidFileName: cfg.PidFileName,
		PidFilePerm: 0600,
		LogFileName: cfg.LogFileName,
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

	go a.worker(ctx, cfg)

	err = daemon.ServeSignals()
	if err != nil {
		log.Printf("error: %s", err.Error())
	}

	log.Println("luks-vault daemon terminated")
}

func (a *Instances) runForeground(ctx context.Context, cfg *config.Configuration) {
	log.Println("luks-vault started in foreground mode")
	if cfg.CryptsetupUseSudo {
		log.Println("cryptsetup will run via sudo")
	}

	sigCh := make(chan os.Signal, 1)
	ossignal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

	go a.worker(ctx, cfg)

	sig := <-sigCh
	log.Printf("received signal %v, shutting down", sig)
	stop <- struct{}{}
	<-done
	log.Println("luks-vault terminated")
}

func (a *Instances) worker(ctx context.Context, cfg *config.Configuration) {
	defer func() {
		done <- struct{}{}
	}()

	vaultConfig := vault.DefaultConfig()
	vaultConfig.Address = cfg.VaultAddress

	client, err := vault.NewClient(vaultConfig)
	if err != nil {
		log.Printf("unable to initialize Vault client: %v", err)
		return
	}
	a.Vault.VaultClient = client
	a.Vault.VaultClient.SetToken(cfg.VaultToken)

	for {
		if stopped := a.waitOrStop(ctx, pollInterval); stopped {
			return
		}

		vaultData, err := a.Vault.GetSecret(ctx, cfg.VaultMountPath, cfg.VaultSecretPath)
		if err != nil {
			log.Printf("unable to get vault secret %s/%s: %v", cfg.VaultMountPath, cfg.VaultSecretPath, err)
			continue
		}

		expired, err := isTTLExpired(vaultData.Created, vaultData.TTL, time.Now())
		if err != nil {
			log.Printf("invalid TTL metadata in vault secret: %v", err)
			continue
		}
		if !expired {
			continue
		}

		if err := a.rotatePassphrase(ctx, cfg, vaultData); err != nil {
			log.Printf("passphrase rotation failed: %v", err)
		}
	}
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

func (a *Instances) rotatePassphrase(ctx context.Context, cfg *config.Configuration, vaultData *module.VaultData) error {
	newKeyPath := fmt.Sprintf("%s/%s", cfg.FolderPassPhrasePath, newKeyName)
	currentKeyPath := fmt.Sprintf("%s/%s", cfg.FolderPassPhrasePath, currentKeyName)
	cleanupNewKey := true
	defer func() {
		if cleanupNewKey {
			if err := os.Remove(newKeyPath); err != nil && !os.IsNotExist(err) {
				log.Printf("warning: failed to remove temporary key file %s: %v", newKeyPath, err)
			}
		}
	}()

	newSlot, err := alternateSlot(vaultData.Slot)
	if err != nil {
		return err
	}

	pwd, err := password.Generate(64, 10, 10, false, false)
	if err != nil {
		return fmt.Errorf("generate new passphrase: %w", err)
	}

	newKeyData := module.VaultData{
		Key:  pwd,
		Slot: newSlot,
		TTL:  vaultData.TTL,
	}

	if err := a.writeKeyFile(newKeyPath, newKeyData); err != nil {
		return fmt.Errorf("write temporary key file: %w", err)
	}

	log.Printf("begin add new passphrase in %s device at keyslot %s", cfg.DevicePath, newKeyData.Slot)
	if _, err := a.Luks.AddPasswdLUKS(cfg.DevicePath, currentKeyPath, newKeyPath, newKeyData.Slot); err != nil {
		return fmt.Errorf("add LUKS key on slot %s: %w", newKeyData.Slot, err)
	}
	log.Printf("new passphrase added in %s device at keyslot %s", cfg.DevicePath, newKeyData.Slot)

	if _, err := a.Luks.VerifyPasswdLUKS(cfg.DevicePath, newKeyPath); err != nil {
		return fmt.Errorf("verify new passphrase with %s: %w", newKeyPath, err)
	}
	log.Printf("verified new passphrase on keyslot %s in %s device", newKeyData.Slot, cfg.DevicePath)

	if err := a.writeKeyFile(currentKeyPath, newKeyData); err != nil {
		return fmt.Errorf("update current key file %s: %w", currentKeyPath, err)
	}
	log.Printf("updated current key file %s", currentKeyPath)

	if err := a.Vault.WriteSecret(ctx, newKeyData, cfg.VaultMountPath, cfg.VaultSecretPath); err != nil {
		return fmt.Errorf("write secret to vault %s/%s: %w", cfg.VaultMountPath, cfg.VaultSecretPath, err)
	}
	log.Printf("new passphrase wrote to Vault server on %s/%s", cfg.VaultMountPath, cfg.VaultSecretPath)

	if _, err := a.Luks.KillKeySlot(cfg.DevicePath, vaultData.Slot, newKeyPath); err != nil {
		return fmt.Errorf("remove old key slot %s: %w", vaultData.Slot, err)
	}
	log.Printf("old passphrase on %s device at keyslot %s removed", cfg.DevicePath, vaultData.Slot)

	cleanupNewKey = false
	if err := os.Remove(newKeyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove temporary key file %s: %w", newKeyPath, err)
	}

	log.Printf("finished change passphrase")
	return nil
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
	log.Println("configuration reload is not supported yet")
	return nil
}
