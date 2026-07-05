#!/bin/sh
set -e

ensure_user() {
	if ! id luks-vault >/dev/null 2>&1; then
		useradd --system --no-create-home --shell /usr/sbin/nologin luks-vault
	fi
}

ensure_dirs() {
	mkdir -p /etc/data-at-rest /var/log/luks-vault /var/lib/luks-vault
	chown root:luks-vault /etc/data-at-rest
	chmod 750 /etc/data-at-rest
	chown luks-vault:luks-vault /var/log/luks-vault
	chmod 750 /var/log/luks-vault
	chown luks-vault:luks-vault /var/lib/luks-vault
	chmod 750 /var/lib/luks-vault

	if [ ! -f /etc/data-at-rest/key ]; then
		install -o root -g luks-vault -m 640 /dev/null /etc/data-at-rest/key
	fi
}

ensure_config() {
	if [ ! -f /etc/luks-vault/config.yaml ]; then
		cp /etc/luks-vault/config.yaml.example /etc/luks-vault/config.yaml
	fi
	chown luks-vault:luks-vault /etc/luks-vault/config.yaml
	chmod 600 /etc/luks-vault/config.yaml
}

validate_sudoers() {
	if [ -f /etc/sudoers.d/luks-vault ] && command -v visudo >/dev/null 2>&1; then
		visudo -cf /etc/sudoers.d/luks-vault >/dev/null
	fi
}

case "$1" in
configure|1)
	ensure_user
	ensure_dirs
	ensure_config
	validate_sudoers

	if command -v systemctl >/dev/null 2>&1; then
		systemctl daemon-reload
		systemctl enable luks-vault.service >/dev/null 2>&1 || true
	fi
	;;
esac

exit 0
