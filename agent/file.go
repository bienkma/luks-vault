package agent

import (
	"github.com/bienkma/luks-vault/module"
	"os"
)

func (a *Instances) writeKeyFile(filePath string, data module.VaultData) error {
	return os.WriteFile(filePath, []byte(data.Key), 0600)
}
