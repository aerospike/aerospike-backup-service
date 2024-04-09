#!/bin/bash -e
WORKSPACE="$(git rev-parse --show-toplevel)"
NEXT_VERSION="$1"
PREV_VERSION="$(cat "$WORKSPACE"/VERSION)"
CHANGE_LOG_RECORD_TEMP_FILE="$(mktemp)"
CHANGE_LOG_TEMP_FILE="$(mktemp)"
TEMP_FILE="$(mktemp)"

sed -i "s/$PREV_VERSION/$NEXT_VERSION/g" "$WORKSPACE"/VERSION

cat <<EOF > "$CHANGE_LOG_RECORD_TEMP_FILE"
aerospike-backup-service ($NEXT_VERSION-1) unstable; urgency=low
    * Version $NEXT_VERSION release
 -- $(git config user.name) <$(git config user.email)>  $(date +'%a, %d %b %Y %H:%M:%S %z')
EOF

cat "$CHANGE_LOG_RECORD_TEMP_FILE" "$WORKSPACE"/packages/deb/debian/changelog > "$CHANGE_LOG_TEMP_FILE"
mv "$CHANGE_LOG_TEMP_FILE" "$WORKSPACE"/packages/deb/debian/changelog

tac "$WORKSPACE"/docs/docs.go | sed "s/$PREV_VERSION/$NEXT_VERSION" | tac > "$TEMP_FILE"
mv "$TEMP_FILE" "$WORKSPACE"/docs/docs.go

sed -i "s/$PREV_VERSION/$NEXT_VERSION" "$WORKSPACE"/internal/server/server.go
sed -i "s/$PREV_VERSION/$NEXT_VERSION" "$WORKSPACE"/internal/util/version.go

bash -c "$WORKSPACE"/scripts/generate_OpenApi.sh
