#!/bin/bash -e
WORKSPACE="$(git rev-parse --show-toplevel)"
NEXT_VERSION="$1"
PREV_VERSION="$(cat "$WORKSPACE"/VERSION)"

PREV_IMAGE_TAG="${PREV_VERSION#v}"
NEXT_IMAGE_TAG="${NEXT_VERSION#v}"

docker run --rm --interactive --volume "$WORKSPACE":/local bash:latest <<EOF
sed -i "s/$PREV_VERSION/$NEXT_VERSION/g" /local/VERSION
sed -i "s/$PREV_VERSION/$NEXT_VERSION/" /local/internal/server/handlers/info.go
sed -i "s|aerospike/aerospike-backup-service:$PREV_IMAGE_TAG|aerospike/aerospike-backup-service:$NEXT_IMAGE_TAG|" /local/build/docker-compose/docker-compose.yaml
EOF

make -C "$WORKSPACE" docs
