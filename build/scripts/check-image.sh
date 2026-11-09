#!/bin/sh
# Sanity-check a built aerospike-backup-service image.
#
# Usage: build/scripts/check-image.sh <image>
#
# Fails if an SSH server ships in the image (the image must never run sshd; see the
# "*sshole" OpenSSH-on-musl advisory, https://sshhole.com), if the binary is not
# root-owned, or if the config is not owned by absuser:abgroup (65532:65532).
set -e

IMAGE="${1:?usage: $0 <image>}"

docker run --rm --user root --entrypoint sh "$IMAGE" -c '
  rc=0
  if apk info 2>/dev/null | grep -qi openssh; then
    echo "::error::openssh package found in image"; rc=1
  fi
  if command -v sshd >/dev/null 2>&1 || [ -e /usr/sbin/sshd ]; then
    echo "::error::sshd binary found in image"; rc=1
  fi
  check_owner() {
    owner="$(stat -c %u:%g "$1")"
    if [ "$owner" != "$2" ]; then
      echo "::error::$1 is owned by $owner, expected $2"; rc=1
    fi
  }
  # Root-owned so the service cannot replace its own executable.
  check_owner /usr/bin/aerospike-backup-service 0:0
  # Owned by absuser:abgroup because the service rewrites it on config updates.
  check_owner /etc/aerospike-backup-service/aerospike-backup-service.yml 65532:65532
  [ "$rc" -eq 0 ] && echo "image checks passed"
  exit "$rc"
'
