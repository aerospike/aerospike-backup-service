#!/bin/bash -e
WORKSPACE="$(git rev-parse --show-toplevel)"
NEXT_VERSION="$1"
PREV_VERSION="$(cat "$WORKSPACE"/VERSION)"
CHANGE_LOG_RECORD_TEMP_FILE="$(mktemp)"
CHANGE_LOG_TEMP_FILE="$(mktemp)"


cat <<EOF > "$CHANGE_LOG_RECORD_TEMP_FILE"
aerospike-backup-service ($NEXT_VERSION-1) unstable; urgency=low
    * Version $NEXT_VERSION release
 -- $(git config user.name) <$(git config user.email)>  $(date +'%a, %d %b %Y %H:%M:%S %z')
EOF

cat "$CHANGE_LOG_RECORD_TEMP_FILE" "$WORKSPACE"/packages/deb/debian/changelog > "$CHANGE_LOG_TEMP_FILE"
mv "$CHANGE_LOG_TEMP_FILE" "$WORKSPACE"/packages/deb/debian/changelog

docker run --rm --interactive --volume "$WORKSPACE":/local bash:latest <<EOF
sed -i "s/$PREV_VERSION/$NEXT_VERSION/g" /local/VERSION
sed -i "s/$PREV_VERSION/$NEXT_VERSION/" /local/internal/server/info.go
EOF

bash -c "$WORKSPACE"/scripts/generate-openapi.sh
