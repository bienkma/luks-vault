package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/bienkma/luks-vault/config"
	"github.com/bienkma/luks-vault/module"
	"github.com/sethvargo/go-password/password"
)

func (a *Instances) rotatePassphrase(ctx context.Context, cfg *config.Configuration, vaultData *module.VaultData) error {
	state, err := loadRotationState(cfg.Agent.StateFile)
	if err != nil {
		return err
	}
	if state != nil {
		if state.OldSlot != vaultData.Slot {
			return fmt.Errorf("rotation state old slot %s does not match vault slot %s", state.OldSlot, vaultData.Slot)
		}
		log.Printf("resuming rotation from step %s", state.Step)
		return a.executeRotation(ctx, cfg, vaultData, state)
	}
	return a.executeRotation(ctx, cfg, vaultData, nil)
}

func (a *Instances) executeRotation(ctx context.Context, cfg *config.Configuration, vaultData *module.VaultData, state *RotationState) error {
	newKeyPath := fmt.Sprintf("%s/%s", cfg.LUKS.PassphraseDir, newKeyName)
	currentKeyPath := fmt.Sprintf("%s/%s", cfg.LUKS.PassphraseDir, currentKeyName)

	if state == nil {
		newSlot, err := alternateSlot(vaultData.Slot)
		if err != nil {
			return err
		}

		state = newRotationState(vaultData.Slot, newSlot, newKeyPath, currentKeyPath, vaultData.TTL)
	}

	newKeyData := module.VaultData{
		Slot: state.NewSlot,
		TTL:  state.TTL,
	}

	if rotationStepComplete(state.Step, rotationStepNewKeyFile) {
		key, err := readPassphraseFile(state.NewKeyPath)
		if err != nil {
			return err
		}
		newKeyData.Key = key
	} else {
		pwd, err := password.Generate(64, 10, 10, false, false)
		if err != nil {
			return fmt.Errorf("generate new passphrase: %w", err)
		}
		newKeyData.Key = pwd

		if err := a.writeKeyFile(state.NewKeyPath, newKeyData); err != nil {
			return fmt.Errorf("write temporary key file: %w", err)
		}
		state.Step = rotationStepNewKeyFile
		if err := saveRotationState(cfg.Agent.StateFile, state); err != nil {
			return err
		}
	}

	if !rotationStepComplete(state.Step, rotationStepLuksAddKey) {
		log.Printf("begin add new passphrase in %s device at keyslot %s", cfg.LUKS.Device, newKeyData.Slot)
		if _, err := a.Luks.AddPasswdLUKS(cfg.LUKS.Device, state.CurrentKeyPath, state.NewKeyPath, newKeyData.Slot); err != nil {
			return fmt.Errorf("add LUKS key on slot %s: %w", newKeyData.Slot, err)
		}
		state.Step = rotationStepLuksAddKey
		if err := saveRotationState(cfg.Agent.StateFile, state); err != nil {
			return err
		}
		log.Printf("new passphrase added in %s device at keyslot %s", cfg.LUKS.Device, newKeyData.Slot)
	}

	if !rotationStepComplete(state.Step, rotationStepLuksVerify) {
		if _, err := a.Luks.VerifyPasswdLUKS(cfg.LUKS.Device, state.NewKeyPath); err != nil {
			return fmt.Errorf("verify new passphrase with %s: %w", state.NewKeyPath, err)
		}
		state.Step = rotationStepLuksVerify
		if err := saveRotationState(cfg.Agent.StateFile, state); err != nil {
			return err
		}
		log.Printf("verified new passphrase on keyslot %s in %s device", newKeyData.Slot, cfg.LUKS.Device)
	}

	if !rotationStepComplete(state.Step, rotationStepCurrentKey) {
		if err := a.writeKeyFile(state.CurrentKeyPath, newKeyData); err != nil {
			return fmt.Errorf("update current key file %s: %w", state.CurrentKeyPath, err)
		}
		state.Step = rotationStepCurrentKey
		if err := saveRotationState(cfg.Agent.StateFile, state); err != nil {
			return err
		}
		log.Printf("updated current key file %s", state.CurrentKeyPath)
	}

	if !rotationStepComplete(state.Step, rotationStepVaultWrite) {
		if err := a.Vault.WriteSecret(ctx, newKeyData, cfg.Vault.KV2Mount, cfg.Vault.SecretPath); err != nil {
			return fmt.Errorf("write secret to vault %s/%s: %w", cfg.Vault.KV2Mount, cfg.Vault.SecretPath, err)
		}
		state.Step = rotationStepVaultWrite
		if err := saveRotationState(cfg.Agent.StateFile, state); err != nil {
			return err
		}
		log.Printf("new passphrase wrote to Vault server on %s/%s", cfg.Vault.KV2Mount, cfg.Vault.SecretPath)
	}

	if !rotationStepComplete(state.Step, rotationStepKillOldSlot) {
		if _, err := a.Luks.KillKeySlot(cfg.LUKS.Device, state.OldSlot, state.NewKeyPath); err != nil {
			return fmt.Errorf("remove old key slot %s: %w", state.OldSlot, err)
		}
		state.Step = rotationStepKillOldSlot
		if err := saveRotationState(cfg.Agent.StateFile, state); err != nil {
			return err
		}
		log.Printf("old passphrase on %s device at keyslot %s removed", cfg.LUKS.Device, state.OldSlot)
	}

	if err := os.Remove(state.NewKeyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove temporary key file %s: %w", state.NewKeyPath, err)
	}
	if err := clearRotationState(cfg.Agent.StateFile); err != nil {
		return err
	}

	log.Printf("finished change passphrase")
	return nil
}

func readPassphraseFile(path string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read passphrase file %s: %w", path, err)
	}
	passphrase := strings.TrimSpace(string(content))
	if passphrase == "" {
		return "", fmt.Errorf("passphrase file %s is empty", path)
	}
	return passphrase, nil
}
