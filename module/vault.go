package module

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bienkma/luks-vault/config"
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

func (v *VaultAgent) Configure(cfg config.VaultConfig) error {
	client, err := newVaultClient(cfg)
	if err != nil {
		return err
	}

	v.VaultClient = client
	return v.authenticate(cfg)
}

func newVaultClient(cfg config.VaultConfig) (*vault.Client, error) {
	vaultConfig := vault.DefaultConfig()
	vaultConfig.Address = cfg.Address

	tlsConfig, err := buildTLSConfig(cfg.TLS)
	if err != nil {
		return nil, err
	}
	if err := vaultConfig.ConfigureTLS(tlsConfig); err != nil {
		return nil, fmt.Errorf("configure vault TLS: %w", err)
	}

	client, err := vault.NewClient(vaultConfig)
	if err != nil {
		return nil, fmt.Errorf("create vault client: %w", err)
	}
	return client, nil
}

func buildTLSConfig(cfg config.TLSConfig) (*vault.TLSConfig, error) {
	if cfg.CACert == "" && cfg.ClientCert == "" && cfg.ClientKey == "" && !cfg.SkipVerify {
		return &vault.TLSConfig{}, nil
	}

	if cfg.ClientCert != "" || cfg.ClientKey != "" {
		if cfg.ClientCert == "" || cfg.ClientKey == "" {
			return nil, fmt.Errorf("vault TLS client cert and key must both be set")
		}
	}

	return &vault.TLSConfig{
		CACert:     cfg.CACert,
		ClientCert: cfg.ClientCert,
		ClientKey:  cfg.ClientKey,
		Insecure:   cfg.SkipVerify,
	}, nil
}

func (v *VaultAgent) authenticate(cfg config.VaultConfig) error {
	switch cfg.AuthMethod {
	case "token":
		v.VaultClient.SetToken(cfg.Token)
		return nil
	case "approle":
		secretID, err := resolveAppRoleSecretID(cfg.AppRole)
		if err != nil {
			return err
		}

		secret, err := v.VaultClient.Logical().Write("auth/approle/login", map[string]interface{}{
			"role_id":   cfg.AppRole.RoleID,
			"secret_id": secretID,
		})
		if err != nil {
			return fmt.Errorf("approle login: %w", err)
		}
		if secret == nil || secret.Auth == nil || secret.Auth.ClientToken == "" {
			return fmt.Errorf("approle login returned empty token")
		}

		v.VaultClient.SetToken(secret.Auth.ClientToken)
		return nil
	default:
		return fmt.Errorf("unsupported vault auth method %q", cfg.AuthMethod)
	}
}

func resolveAppRoleSecretID(cfg config.AppRoleConfig) (string, error) {
	if strings.TrimSpace(cfg.SecretID) != "" {
		return strings.TrimSpace(cfg.SecretID), nil
	}

	content, err := os.ReadFile(cfg.SecretIDFile)
	if err != nil {
		return "", fmt.Errorf("read vault.approle.secret_id_file: %w", err)
	}

	secretID := strings.TrimSpace(string(content))
	if secretID == "" {
		return "", fmt.Errorf("vault.approle.secret_id_file is empty")
	}
	return secretID, nil
}

func (v *VaultAgent) GetSecret(ctx context.Context, kv2Mount, secretPath string) (*VaultData, error) {
	vaultResponse, err := v.VaultClient.KVv2(kv2Mount).Get(ctx, secretPath)
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}

	return parseVaultData(vaultResponse.Data)
}

func (v *VaultAgent) WriteSecret(ctx context.Context, vaultData VaultData, kv2Mount, secretPath string) error {
	secretData := map[string]interface{}{
		"key":     vaultData.Key,
		"ttl":     vaultData.TTL,
		"slot":    vaultData.Slot,
		"created": time.Now().Format(time.RFC3339),
	}
	_, err := v.VaultClient.KVv2(kv2Mount).Put(ctx, secretPath, secretData)
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
	case json.Number:
		return typed.String(), nil
	default:
		return "", fmt.Errorf("field %q has unexpected type %T", name, value)
	}
}
