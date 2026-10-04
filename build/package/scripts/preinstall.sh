#!/bin/bash -e
#
# Runs before any file is unpacked, so the packaged files can be owned by the service
# account from the moment they land and nothing needs a recursive chown afterwards.

SERVICE_USER=aerospike-backup-service
SERVICE_GROUP=aerospike-backup-service
STATE_DIR=/var/lib/aerospike-backup-service

if ! getent group "${SERVICE_GROUP}" >/dev/null; then
	groupadd --system "${SERVICE_GROUP}"
fi

if ! getent passwd "${SERVICE_USER}" >/dev/null; then
	useradd --system --gid "${SERVICE_GROUP}" --no-create-home \
		--home-dir "${STATE_DIR}" --shell /usr/sbin/nologin "${SERVICE_USER}"
fi
