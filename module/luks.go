package module

import (
	"fmt"
	"os/exec"
)

type LUKSOperation struct {
	UseSudo bool
}

const (
	luksCmd    = "/usr/sbin/cryptsetup"
	sudoCmd    = "/usr/bin/sudo"
	sudoNonInt = "-n"
)

func (l *LUKSOperation) runCryptsetup(args ...string) ([]byte, error) {
	if l.UseSudo {
		cmdArgs := append([]string{sudoNonInt, luksCmd}, args...)
		out, err := exec.Command(sudoCmd, cmdArgs...).CombinedOutput()
		if err != nil {
			return out, fmt.Errorf("sudo cryptsetup: %w: %s", err, string(out))
		}
		return out, nil
	}

	out, err := exec.Command(luksCmd, args...).CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("cryptsetup: %w: %s", err, string(out))
	}
	return out, nil
}

func (l *LUKSOperation) AddPasswdLUKS(devicePath, currentPassphraseKeyPath, newPassphraseKeyPath, newKeySlot string) ([]byte, error) {
	return l.runCryptsetup("-v", "-q", "luksAddKey", devicePath, newPassphraseKeyPath, "-d", currentPassphraseKeyPath, "-S", newKeySlot)
}

func (l *LUKSOperation) KillKeySlot(devicePath, keySlot, passphraseKeyPath string) ([]byte, error) {
	return l.runCryptsetup("-q", "-v", "luksKillSlot", devicePath, keySlot, "-d", passphraseKeyPath)
}

func (l *LUKSOperation) VerifyPasswdLUKS(devicePath, newPassphraseKeyPath string) ([]byte, error) {
	return l.runCryptsetup("-q", "-v", "open", "--test-passphrase", "--type", "luks", devicePath, "-d", newPassphraseKeyPath)
}
