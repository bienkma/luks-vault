#!/bin/sh
set -e

case "$1" in
remove|deconfigure|purge|upgrade|0)
	if command -v systemctl >/dev/null 2>&1; then
		systemctl stop luks-vault.service >/dev/null 2>&1 || true
		systemctl disable luks-vault.service >/dev/null 2>&1 || true
		systemctl daemon-reload >/dev/null 2>&1 || true
	fi
	;;
esac

exit 0
