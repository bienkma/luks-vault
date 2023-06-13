package config

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
)

type Configuration struct {
	VaultAddress         string
	VaultToken           string
	VaultMountPath       string
	VaultSecretPath      string
	ModuleLuks           bool
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
	err := vip.ReadInConfig()
	if err != nil {
		fmt.Println("fatal error config file: config.yaml \n", err)
		os.Exit(1)
	}
	ServerConfig := &Configuration{
		VaultAddress:         vip.GetString("vault_address"),
		VaultToken:           vip.GetString("vault_token"),
		VaultMountPath:       vip.GetString("vault_mount_path"),
		VaultSecretPath:      vip.GetString("vault_secret_path"),
		ModuleLuks:           vip.GetBool("vault_module_luks"),
		DevicePath:           vip.GetString("device_path"),
		FolderPassPhrasePath: vip.GetString("folder_pass_phrase_path"),
		PidFileName:          vip.GetString("pid_file_name"),
		LogFileName:          vip.GetString("log_file_name"),
	}
	return ServerConfig
}
