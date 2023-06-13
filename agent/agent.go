package agent

import (
	"context"
	"flag"
	"fmt"
	"github.com/bienkma/luks-vault/config"
	"github.com/bienkma/luks-vault/module"
	vault "github.com/hashicorp/vault/api"
	"github.com/sethvargo/go-password/password"
	"github.com/sevlyar/go-daemon"
	"log"
	"os"
	"syscall"
	"time"
)

var (
	signal = flag.String("s", "", `Send signal to the daemon:
	quit - graceful shutdown
	stop - fast shutdown
	reload - reloading the configuration file`)
	stop           = make(chan struct{})
	done           = make(chan struct{})
	currentKeyName = "key"
	oldKeyName     = "oldKey"
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
	// Load configuration agent
	cfg := config.New()

	// Daemon load configuration
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "quit"), syscall.SIGQUIT, termHandle)
	daemon.AddCommand(daemon.StringFlag(signal, "reload"), syscall.SIGHUP, reloadHandler)

	cntxt := &daemon.Context{
		PidFileName: cfg.PidFileName,
		PidFilePerm: 0644,
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

func (a *Instances) worker(ctx context.Context, cfg *config.Configuration) {
	// vault client init
	config := vault.DefaultConfig()

	config.Address = cfg.VaultAddress

	client, err := vault.NewClient(config)
	if err != nil {
		log.Fatalf("unable to initialize Vault client: %v\n", err)
	}
	a.Vault.VaultClient = client

	// Authenticate
	a.Vault.VaultClient.SetToken(cfg.VaultToken)
LOOP:
	for {
		time.Sleep(10 * time.Second) // this is work to be done by worker.
		// step 1: Get information from vault
		vaultData, secErr := a.Vault.GetSecret(ctx, cfg.VaultMountPath, cfg.VaultSecretPath)
		if secErr != nil {
			log.Fatalf("step 1: unable to get vault %s/%s \n", cfg.VaultMountPath, cfg.VaultSecretPath)
		}
		now := time.Now()
		created, _ := time.Parse(time.RFC3339, vaultData.Created)
		ttl, _ := time.ParseDuration(vaultData.TTL)

		// step 2: check TTL
		if created.Add(ttl).Before(now) {
			var (
				//oldKeyPath     = fmt.Sprintf("%s/%s", cfg.FolderPassPhrasePath, oldKeyName)
				newKeyPath     = fmt.Sprintf("%s/%s", cfg.FolderPassPhrasePath, newKeyName)
				currentKeyPath = fmt.Sprintf("%s/%s", cfg.FolderPassPhrasePath, currentKeyName)
			)

			oldKeyData := module.VaultData{
				Key:     vaultData.Key,
				Slot:    vaultData.Slot,
				TTL:     vaultData.TTL,
				Created: vaultData.Created,
			}

			// step 3: create new password
			pwd, _ := password.Generate(64, 10, 10, false, false)
			newKeyData := module.VaultData{
				Key:     pwd,
				Slot:    vaultData.Slot,
				TTL:     vaultData.TTL,
				Created: vaultData.Created,
			}

			// step 4: write new file key
			if oldKeyData.Slot == "0" {
				newKeyData.Slot = "1"
			} else {
				newKeyData.Slot = "0"
			}
			if err := a.writeKeyFile(newKeyPath, newKeyData); err != nil {
				log.Printf("step 4: unable to write %s/newKeyData\n", cfg.FolderPassPhrasePath)
				log.Fatalln(err)
			}
			// step 5: update newKey to LUKS
			log.Printf("begin add new passphrase in %s device at keyslot %s\n", cfg.DevicePath, newKeyData.Slot)
			_, errAdd := a.Luks.AddPasswdLUKS(cfg.DevicePath, currentKeyPath, newKeyPath, newKeyData.Slot)
			if errAdd != nil {
				log.Fatalf("step 5: we can not add luks to device %s with newkey %s at keyslot %s", cfg.DevicePath, newKeyPath, newKeyData.Slot)
			}
			log.Printf("new passphrase has added in %s device at keyslot %s \n", cfg.DevicePath, newKeyData.Slot)

			// step 6: verify passPhrase with LUKS device
			_, errVerify := a.Luks.VerifyPasswdLUKS(cfg.DevicePath, newKeyPath)
			if errVerify != nil {
				log.Fatalf("step 6: we can not change passPharse on the device with %s key\n", newKeyPath)
			}
			log.Printf("verify new passphrase on keyslot %s in %s device\n", newKeyData.Slot, cfg.DevicePath)
			// step 7: write passPhrase to Vault
			if err := a.Vault.WriteSecret(ctx, newKeyData, cfg.VaultMountPath, cfg.VaultSecretPath); err != nil {
				log.Fatalf("step 7: we can not write data to vault %v", err)
			}
			log.Printf("new passphrase wrote to Vault server on %s/%s", cfg.VaultMountPath, cfg.VaultSecretPath)
			// step 8: remove old key slot
			_, err := a.Luks.KillKeySlot(cfg.DevicePath, oldKeyData.Slot, newKeyPath)
			if err != nil {
				log.Fatalf("step 8: we can not remove old key slot %s", oldKeyData.Slot)
			}
			log.Printf("old passpharse on %s device at keyslot %s removed", cfg.DevicePath, oldKeyData.Slot)
			// step 9: update current key
			errW := a.writeKeyFile(currentKeyPath, newKeyData)
			if errW != nil {
				log.Fatalf("step 9: can not update newKey to current key")
			}
			log.Printf("finished change passphrase!...")
		}

		select {
		case <-stop:
			break LOOP
		default:
		}
	}
	done <- struct{}{}
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
	log.Println("configuration reloaded")
	return nil
}
