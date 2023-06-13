package module

import (
	"os/exec"
)

type LUKSOperation struct {
	arg string
}

const (
	luksCmd = "/usr/sbin/cryptsetup"
)

func (l *LUKSOperation) AddPasswdLUKS(devicePath, currentPassphraseKeyPath, newPassphraseKeyPath, newKeySlot string) ([]byte, error) {
	return exec.Command(luksCmd, "-v", "-q", "luksAddKey", devicePath, newPassphraseKeyPath, "-d", currentPassphraseKeyPath, "-S", newKeySlot).Output()
}

func (l *LUKSOperation) KillKeySlot(devicePath, keySlot, PassphraseKeyPath string) ([]byte, error) {
	return exec.Command(luksCmd, "-q", "-v", "luksKillSlot", devicePath, keySlot, "-d", PassphraseKeyPath).Output()
}

func (l *LUKSOperation) VerifyPasswdLUKS(devicePath, newPassphraseKeyPath string) ([]byte, error) {
	return exec.Command(luksCmd, "-q", "-v", "open", "--test-passphrase", "--type", "luks", devicePath, "-d", newPassphraseKeyPath).Output()
}
