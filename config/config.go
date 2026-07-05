package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

type Configuration struct {
	VaultAddress         string
	VaultToken           string
	VaultMountPath       string
	VaultSecretPath      string
	ModuleLuks           bool
	CryptsetupUseSudo    bool
	DevicePath           string
	FolderPassPhrasePath string
	PidFileName          string
	LogFileName          string
}

func New() *Configuration {
	vip := viper.New()
	vip.SetConfigName("config")
	vip.SetConfigType("yaml")
	vip.AddConfigPath("/etc/luks-vault/")
	vip.AutomaticEnv()
	if err := vip.ReadInConfig(); err != nil {
		fmt.Println("fatal error config file: config.yaml \n", err)
		os.Exit(1)
	}

	cfg := &Configuration{
		VaultAddress:         vip.GetString("vault_address"),
		VaultToken:           vip.GetString("vault_token"),
		VaultMountPath:       vip.GetString("vault_mount_path"),
		VaultSecretPath:      vip.GetString("vault_secret_path"),
		ModuleLuks:           vip.GetBool("vault_module_luks"),
		CryptsetupUseSudo:    vip.GetBool("cryptsetup_use_sudo"),
		DevicePath:           vip.GetString("device_path"),
		FolderPassPhrasePath: vip.GetString("folder_pass_phrase_path"),
		PidFileName:          vip.GetString("pid_file_name"),
		LogFileName:          vip.GetString("log_file_name"),
	}
	if err := cfg.Validate(); err != nil {
		fmt.Println("fatal error config validation:", err)
		os.Exit(1)
	}
	return cfg
}

func (c *Configuration) Validate() error {
	required := map[string]string{
		"vault_address":           c.VaultAddress,
		"vault_token":             c.VaultToken,
		"vault_mount_path":        c.VaultMountPath,
		"vault_secret_path":       c.VaultSecretPath,
		"folder_pass_phrase_path": c.FolderPassPhrasePath,
		"pid_file_name":           c.PidFileName,
		"log_file_name":           c.LogFileName,
	}
	for name, value := range required {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if c.ModuleLuks && strings.TrimSpace(c.DevicePath) == "" {
		return fmt.Errorf("device_path is required when vault_module_luks is enabled")
	}
	return nil
}
