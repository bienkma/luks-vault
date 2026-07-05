package module

import (
	"context"
	"fmt"
	"time"

	vault "github.com/hashicorp/vault/api"
)

type VaultAgent struct {
	VaultClient *vault.Client
}

type VaultData struct {
	Key     string
	TTL     string
	Slot    string
	Created string
}

func (v *VaultAgent) GetSecret(ctx context.Context, vaultSecretPath, secretName string) (*VaultData, error) {
	vaultResponse, err := v.VaultClient.KVv2(vaultSecretPath).Get(ctx, secretName)
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}

	return parseVaultData(vaultResponse.Data)
}

func (v *VaultAgent) WriteSecret(ctx context.Context, vaultData VaultData, vaultSecretPath, secretName string) error {
	secretData := map[string]interface{}{
		"key":     vaultData.Key,
		"ttl":     vaultData.TTL,
		"slot":    vaultData.Slot,
		"created": time.Now().Format(time.RFC3339),
	}
	_, err := v.VaultClient.KVv2(vaultSecretPath).Put(ctx, secretName, secretData)
	if err != nil {
		return fmt.Errorf("write secret: %w", err)
	}
	return nil
}

func parseVaultData(raw map[string]interface{}) (*VaultData, error) {
	key, err := fieldAsString(raw, "key")
	if err != nil {
		return nil, err
	}
	ttl, err := fieldAsString(raw, "ttl")
	if err != nil {
		return nil, err
	}
	slot, err := fieldAsString(raw, "slot")
	if err != nil {
		return nil, err
	}
	created, err := fieldAsString(raw, "created")
	if err != nil {
		return nil, err
	}

	return &VaultData{
		Key:     key,
		TTL:     ttl,
		Slot:    slot,
		Created: created,
	}, nil
}

func fieldAsString(data map[string]interface{}, name string) (string, error) {
	value, ok := data[name]
	if !ok {
		return "", fmt.Errorf("missing %q in vault secret", name)
	}

	switch typed := value.(type) {
	case string:
		return typed, nil
	case time.Time:
		return typed.Format(time.RFC3339), nil
	default:
		return "", fmt.Errorf("field %q has unexpected type %T", name, value)
	}
}
