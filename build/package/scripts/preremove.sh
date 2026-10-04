#!/bin/bash -e

SERVICE_NAME=aerospike-backup-service

# On an upgrade the package manager runs the *old* package's script after the new one
# has already started the service: rpm passes 1, dpkg passes "upgrade". Stopping and
# disabling there is what left the service down after every rpm upgrade.
case "${1:-}" in
	0 | remove | purge) ;;
	*) exit 0 ;;
esac

if ! command -v systemctl >/dev/null || [ ! -d /run/systemd/system ]; then
	exit 0
fi

systemctl stop "${SERVICE_NAME}" || :
systemctl disable "${SERVICE_NAME}" || :
