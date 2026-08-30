#!/bin/bash -e

SERVICE_USER=aerospike-backup-service
SERVICE_GROUP=aerospike-backup-service
CONFIG_DIR=/etc/aerospike-backup-service
CONFIG_FILE="${CONFIG_DIR}/aerospike-backup-service.yml"
STATE_DIR=/var/lib/aerospike-backup-service
LOG_FILE=/var/log/aerospike-backup-service.log

if ! getent group "${SERVICE_GROUP}" >/dev/null; then
	groupadd --system "${SERVICE_GROUP}"
fi

if ! getent passwd "${SERVICE_USER}" >/dev/null; then
	useradd --system --gid "${SERVICE_GROUP}" --no-create-home \
		--home-dir "${STATE_DIR}" --shell /usr/sbin/nologin "${SERVICE_USER}"
fi

mkdir -p "${CONFIG_DIR}" "${STATE_DIR}"
touch "${LOG_FILE}"

chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "${CONFIG_DIR}" "${STATE_DIR}"
chown "${SERVICE_USER}:${SERVICE_GROUP}" "${LOG_FILE}"
chmod 0640 "${CONFIG_FILE}"
chmod 0600 "${LOG_FILE}"

systemctl daemon-reload
systemctl enable aerospike-backup-service
systemctl start aerospike-backup-service
