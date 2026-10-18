#!/bin/bash -e

SERVICE_USER=aerospike-backup-service
SERVICE_GROUP=aerospike-backup-service

# Only on a real removal, and only for rpm erase or a dpkg purge: "remove" keeps the
# configuration, so it keeps the account that owns it too.
case "${1:-}" in
	0 | purge) ;;
	*) exit 0 ;;
esac

if getent passwd "${SERVICE_USER}" >/dev/null; then
	userdel "${SERVICE_USER}" >/dev/null 2>&1 || :
fi

if getent group "${SERVICE_GROUP}" >/dev/null; then
	groupdel "${SERVICE_GROUP}" >/dev/null 2>&1 || :
fi
