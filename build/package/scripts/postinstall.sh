#!/bin/bash -e

SERVICE_NAME=aerospike-backup-service
SERVICE_USER=aerospike-backup-service
SERVICE_GROUP=aerospike-backup-service
CONFIG_FILE=/etc/aerospike-backup-service/aerospike-backup-service.yml
LEGACY_UNIT=/etc/systemd/system/aerospike-backup-service.service
STATE_DIR=/var/lib/aerospike-backup-service
LOG_DIR=/var/log/aerospike-backup-service
LEGACY_LOG=/var/log/aerospike-backup-service.log

# Releases before this one logged to /var/log/aerospike-backup-service.log and ran as
# root. The unit now uses LogsDirectory=, so move the existing history into the new
# directory instead of stranding it behind ProtectSystem=strict.
if [ -f "${LEGACY_LOG}" ]; then
	mkdir -p "${LOG_DIR}"
	mv "${LEGACY_LOG}" "${LOG_DIR}/${SERVICE_NAME}.log"
	for legacy in "${LEGACY_LOG%.log}"-*.log "${LEGACY_LOG%.log}"-*.log.gz; do
		if [ -e "${legacy}" ]; then
			mv "${legacy}" "${LOG_DIR}/"
		fi
	done
	chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "${LOG_DIR}"
fi

# The configuration file holds cluster passwords and cloud keys. On an upgrade dpkg and
# rpm keep the existing file's owner and mode, so the ownership nfpm applies at unpack
# never reaches an install that predates the service account: enforce it here.
if [ -f "${CONFIG_FILE}" ]; then
	chown "${SERVICE_USER}:${SERVICE_GROUP}" "${CONFIG_FILE}"
	chmod 0600 "${CONFIG_FILE}"
fi

# Earlier releases shipped the unit as a conffile in /etc/systemd/system. A copy left
# there outranks the vendor unit, so an operator who edited it would keep running as
# root with no sandbox. Move that one aside -- a deliberate override carrying User= is
# left alone, and customisation should live in a drop-in instead.
if [ -f "${LEGACY_UNIT}" ] && ! grep -q '^User=' "${LEGACY_UNIT}"; then
	mv "${LEGACY_UNIT}" "${LEGACY_UNIT}.pre-hardening.bak"
	echo "${SERVICE_NAME}: moved the pre-hardening unit ${LEGACY_UNIT} to ${LEGACY_UNIT}.pre-hardening.bak" >&2
	echo "${SERVICE_NAME}: it predates the service account and would have kept the service running as root." >&2
	echo "${SERVICE_NAME}: re-apply any local changes with 'systemctl edit ${SERVICE_NAME}'." >&2
fi

# An install that predates the service account left this state root-owned.
if [ -d "${STATE_DIR}" ]; then
	find "${STATE_DIR}" \! -user "${SERVICE_USER}" -exec \
		chown -h "${SERVICE_USER}:${SERVICE_GROUP}" {} +
fi

# Containers and chroots have no running systemd; the package still installs there.
if ! command -v systemctl >/dev/null || [ ! -d /run/systemd/system ]; then
	echo "systemd is not running: skipping daemon-reload/enable/start for ${SERVICE_NAME}" >&2
	exit 0
fi

systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
systemctl restart "${SERVICE_NAME}"
