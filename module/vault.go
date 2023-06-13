package module

import (
	"context"
	vault "github.com/hashicorp/vault/api"
	"log"
	"time"
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
		log.Printf("read secret with an error %v\n", err)
		return nil, err
	}

	data := &VaultData{
		Key:     vaultResponse.Data["key"].(string),
		TTL:     vaultResponse.Data["ttl"].(string),
		Slot:    vaultResponse.Data["slot"].(string),
		Created: vaultResponse.Data["created"].(string),
	}
	return data, nil
}

func (v *VaultAgent) WriteSecret(ctx context.Context, vaultData VaultData, vaultSecretPath, secretName string) error {
	secretData := map[string]interface{}{
		"key":     vaultData.Key,
		"ttl":     vaultData.TTL,
		"slot":    vaultData.Slot,
		"created": time.Now(),
	}
	_, err := v.VaultClient.KVv2(vaultSecretPath).Put(ctx, secretName, secretData)
	return err
}
