#!/bin/sh
# Sanity-check a built aerospike-backup-service image.
#
# Usage: build/scripts/check-image.sh <image>
#
# Fails if an SSH server ships in the image (the image must never run sshd; see the
# "*sshole" OpenSSH-on-musl advisory, https://sshhole.com) or if the service files are
# not owned by absuser:abgroup (65532:65532).
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
  for f in /usr/bin/aerospike-backup-service /etc/aerospike-backup-service/aerospike-backup-service.yml; do
    owner="$(stat -c %u:%g "$f")"
    if [ "$owner" != "65532:65532" ]; then
      echo "::error::$f is owned by $owner, expected 65532:65532"; rc=1
    fi
  done
  [ "$rc" -eq 0 ] && echo "image checks passed"
  exit "$rc"
'
