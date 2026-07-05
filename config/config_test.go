package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidTokenConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
vault:
  address: "https://vault.example.com:8200"
  auth_method: token
  token: "test-token"
  kv2_mount: "luks/host"
  secret_path: "dev/sda"
  tls:
    ca_cert: "/etc/luks-vault/ca.pem"
luks:
  enabled: true
  device: "/dev/sda"
  passphrase_dir: "/etc/data-at-rest"
agent:
  log_file: "/var/log/luks-vault/agent.log"
  state_file: "/var/lib/luks-vault/rotation.state"
`)

	cfg, err := loadFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Vault.AuthMethod != "token" {
		t.Fatalf("got auth method %q", cfg.Vault.AuthMethod)
	}
}

func TestLoadValidAppRoleConfig(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
vault:
  address: "https://vault.example.com:8200"
  auth_method: approle
  approle:
    role_id: "role-id"
    secret_id_file: "/etc/luks-vault/secret-id"
  kv2_mount: "luks/host"
  secret_path: "dev/sda"
  tls:
    ca_cert: "/etc/luks-vault/ca.pem"
    client_cert: "/etc/luks-vault/client.pem"
    client_key: "/etc/luks-vault/client-key.pem"
luks:
  enabled: true
  device: "/dev/sda"
  passphrase_dir: "/etc/data-at-rest"
agent:
  log_file: "/var/log/luks-vault/agent.log"
`)

	cfg, err := loadFromDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Vault.AppRole.RoleID != "role-id" {
		t.Fatalf("unexpected approle config: %+v", cfg.Vault.AppRole)
	}
}

func TestLoadMissingVaultToken(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
vault:
  address: "https://vault.example.com:8200"
  auth_method: token
  kv2_mount: "luks/host"
  secret_path: "dev/sda"
luks:
  passphrase_dir: "/etc/data-at-rest"
agent:
  log_file: "/var/log/luks-vault/agent.log"
`)

	if _, err := loadFromDir(dir); err == nil {
		t.Fatal("expected validation error")
	}
}

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func loadFromDir(dir string) (*Configuration, error) {
	oldLoad := loadConfigDir
	loadConfigDir = func() []string { return []string{dir} }
	defer func() { loadConfigDir = oldLoad }()
	return Load()
}
