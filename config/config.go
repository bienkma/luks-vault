package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Configuration struct {
	Agent AgentConfig
	Vault VaultConfig
	LUKS  LUKSConfig
}

type AgentConfig struct {
	PollInterval time.Duration
	PIDFile      string
	LogFile      string
	StateFile    string
}

type VaultConfig struct {
	Address    string
	AuthMethod string
	Token      string
	AppRole    AppRoleConfig
	KV2Mount   string
	SecretPath string
	TLS        TLSConfig
}

type AppRoleConfig struct {
	RoleID       string
	SecretID     string
	SecretIDFile string
}

type TLSConfig struct {
	CACert     string
	ClientCert string
	ClientKey  string
	SkipVerify bool
}

type LUKSConfig struct {
	Enabled           bool
	Device            string
	PassphraseDir     string
	CryptsetupUseSudo bool
}

func New() *Configuration {
	cfg, err := Load()
	if err != nil {
		fmt.Println("fatal error loading config:", err)
		os.Exit(1)
	}
	return cfg
}

func Load() (*Configuration, error) {
	vip := viper.New()
	vip.SetConfigName("config")
	vip.SetConfigType("yaml")
	for _, path := range loadConfigDir() {
		vip.AddConfigPath(path)
	}
	vip.AutomaticEnv()

	vip.SetDefault("agent.poll_interval", "10s")
	vip.SetDefault("agent.state_file", "/var/lib/luks-vault/rotation.state")
	vip.SetDefault("vault.auth_method", "token")
	vip.SetDefault("vault.tls.skip_verify", false)
	vip.SetDefault("luks.enabled", true)
	vip.SetDefault("luks.cryptsetup_use_sudo", true)

	if err := vip.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	pollInterval, err := time.ParseDuration(vip.GetString("agent.poll_interval"))
	if err != nil {
		return nil, fmt.Errorf("parse agent.poll_interval: %w", err)
	}

	cfg := &Configuration{
		Agent: AgentConfig{
			PollInterval: pollInterval,
			PIDFile:      vip.GetString("agent.pid_file"),
			LogFile:      vip.GetString("agent.log_file"),
			StateFile:    vip.GetString("agent.state_file"),
		},
		Vault: VaultConfig{
			Address:    vip.GetString("vault.address"),
			AuthMethod: strings.ToLower(strings.TrimSpace(vip.GetString("vault.auth_method"))),
			Token:      vip.GetString("vault.token"),
			AppRole: AppRoleConfig{
				RoleID:       vip.GetString("vault.approle.role_id"),
				SecretID:     vip.GetString("vault.approle.secret_id"),
				SecretIDFile: vip.GetString("vault.approle.secret_id_file"),
			},
			KV2Mount:   vip.GetString("vault.kv2_mount"),
			SecretPath: vip.GetString("vault.secret_path"),
			TLS: TLSConfig{
				CACert:     vip.GetString("vault.tls.ca_cert"),
				ClientCert: vip.GetString("vault.tls.client_cert"),
				ClientKey:  vip.GetString("vault.tls.client_key"),
				SkipVerify: vip.GetBool("vault.tls.skip_verify"),
			},
		},
		LUKS: LUKSConfig{
			Enabled:           vip.GetBool("luks.enabled"),
			Device:            vip.GetString("luks.device"),
			PassphraseDir:     vip.GetString("luks.passphrase_dir"),
			CryptsetupUseSudo: vip.GetBool("luks.cryptsetup_use_sudo"),
		},
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

var loadConfigDir = func() []string {
	return []string{"/etc/luks-vault/"}
}

func (c *Configuration) Validate() error {
	if strings.TrimSpace(c.Vault.Address) == "" {
		return fmt.Errorf("vault.address is required")
	}
	if strings.TrimSpace(c.Vault.KV2Mount) == "" {
		return fmt.Errorf("vault.kv2_mount is required")
	}
	if strings.TrimSpace(c.Vault.SecretPath) == "" {
		return fmt.Errorf("vault.secret_path is required")
	}
	if strings.TrimSpace(c.Agent.LogFile) == "" {
		return fmt.Errorf("agent.log_file is required")
	}
	if strings.TrimSpace(c.Agent.StateFile) == "" {
		return fmt.Errorf("agent.state_file is required")
	}
	if c.Agent.PollInterval <= 0 {
		return fmt.Errorf("agent.poll_interval must be greater than zero")
	}

	switch c.Vault.AuthMethod {
	case "token":
		if strings.TrimSpace(c.Vault.Token) == "" {
			return fmt.Errorf("vault.token is required when vault.auth_method is token")
		}
	case "approle":
		if strings.TrimSpace(c.Vault.AppRole.RoleID) == "" {
			return fmt.Errorf("vault.approle.role_id is required when vault.auth_method is approle")
		}
		if strings.TrimSpace(c.Vault.AppRole.SecretID) == "" && strings.TrimSpace(c.Vault.AppRole.SecretIDFile) == "" {
			return fmt.Errorf("vault.approle.secret_id or vault.approle.secret_id_file is required when vault.auth_method is approle")
		}
	default:
		return fmt.Errorf("vault.auth_method must be token or approle")
	}

	if c.LUKS.Enabled && strings.TrimSpace(c.LUKS.Device) == "" {
		return fmt.Errorf("luks.device is required when luks.enabled is true")
	}
	if strings.TrimSpace(c.LUKS.PassphraseDir) == "" {
		return fmt.Errorf("luks.passphrase_dir is required")
	}
	return nil
}
