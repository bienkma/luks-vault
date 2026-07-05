package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	rotationStepNewKeyFile  = "new_key_file"
	rotationStepLuksAddKey  = "luks_add_key"
	rotationStepLuksVerify  = "luks_verify"
	rotationStepCurrentKey  = "current_key"
	rotationStepVaultWrite  = "vault_write"
	rotationStepKillOldSlot = "kill_old_slot"
)

type RotationState struct {
	Step           string `json:"step"`
	StartedAt      string `json:"started_at"`
	OldSlot        string `json:"old_slot"`
	NewSlot        string `json:"new_slot"`
	NewKeyPath     string `json:"new_key_path"`
	CurrentKeyPath string `json:"current_key_path"`
	TTL            string `json:"ttl"`
}

func loadRotationState(path string) (*RotationState, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read rotation state: %w", err)
	}

	var state RotationState
	if err := json.Unmarshal(content, &state); err != nil {
		return nil, fmt.Errorf("parse rotation state: %w", err)
	}
	if state.Step == "" {
		return nil, fmt.Errorf("rotation state file %s has empty step", path)
	}
	return &state, nil
}

func saveRotationState(path string, state *RotationState) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("create rotation state directory: %w", err)
	}

	content, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode rotation state: %w", err)
	}

	tempFile := path + ".tmp"
	if err := os.WriteFile(tempFile, content, 0600); err != nil {
		return fmt.Errorf("write rotation state temp file: %w", err)
	}
	if err := os.Rename(tempFile, path); err != nil {
		return fmt.Errorf("replace rotation state file: %w", err)
	}
	return nil
}

func clearRotationState(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove rotation state: %w", err)
	}
	return nil
}

func newRotationState(oldSlot, newSlot, newKeyPath, currentKeyPath, ttl string) *RotationState {
	return &RotationState{
		StartedAt:      time.Now().Format(time.RFC3339),
		OldSlot:        oldSlot,
		NewSlot:        newSlot,
		NewKeyPath:     newKeyPath,
		CurrentKeyPath: currentKeyPath,
		TTL:            ttl,
	}
}

func rotationStepComplete(currentStep, completedStep string) bool {
	order := map[string]int{
		rotationStepNewKeyFile:  1,
		rotationStepLuksAddKey:  2,
		rotationStepLuksVerify:  3,
		rotationStepCurrentKey:  4,
		rotationStepVaultWrite:  5,
		rotationStepKillOldSlot: 6,
	}
	return order[currentStep] >= order[completedStep]
}
