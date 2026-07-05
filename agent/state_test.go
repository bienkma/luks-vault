package agent

import (
	"testing"
)

func TestRotationStepComplete(t *testing.T) {
	if !rotationStepComplete(rotationStepVaultWrite, rotationStepLuksVerify) {
		t.Fatal("expected vault_write to be after luks_verify")
	}
	if rotationStepComplete(rotationStepLuksAddKey, rotationStepVaultWrite) {
		t.Fatal("did not expect luks_add_key to be after vault_write")
	}
}

func TestSaveAndLoadRotationState(t *testing.T) {
	path := t.TempDir() + "/rotation.state"
	state := newRotationState("1", "0", "/tmp/newKey", "/tmp/key", "30m")
	state.Step = rotationStepLuksVerify

	if err := saveRotationState(path, state); err != nil {
		t.Fatalf("save state: %v", err)
	}

	loaded, err := loadRotationState(path)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if loaded.Step != rotationStepLuksVerify || loaded.NewSlot != "0" {
		t.Fatalf("unexpected loaded state: %+v", loaded)
	}

	if err := clearRotationState(path); err != nil {
		t.Fatalf("clear state: %v", err)
	}
	loaded, err = loadRotationState(path)
	if err != nil || loaded != nil {
		t.Fatalf("expected empty state after clear, got %+v err=%v", loaded, err)
	}
}
