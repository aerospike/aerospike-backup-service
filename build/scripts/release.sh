#!/bin/bash -e
NEXT_VERSION="$1"

# Without an argument the sed calls below rewrite every occurrence of the current version to
# the empty string, quietly corrupting VERSION, info.go and the compose file.
if [ -z "$NEXT_VERSION" ]; then
  echo "release: NEXT_VERSION is required, e.g." >&2
  echo "  NEXT_VERSION=v3.7.0 NEXT_HELM_CHART_VERSION=2.1.0 make release" >&2
  exit 1
fi

if ! echo "$NEXT_VERSION" | grep -qE '^v[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "release: '$NEXT_VERSION' is not a vX.Y.Z version." >&2
  exit 1
fi

WORKSPACE="$(git rev-parse --show-toplevel)"
PREV_VERSION="$(cat "$WORKSPACE"/VERSION)"

PREV_IMAGE_TAG="${PREV_VERSION#v}"
NEXT_IMAGE_TAG="${NEXT_VERSION#v}"

docker run --rm --interactive --volume "$WORKSPACE":/local bash:latest <<EOF
sed -i "s/$PREV_VERSION/$NEXT_VERSION/g" /local/VERSION
sed -i "s/$PREV_VERSION/$NEXT_VERSION/" /local/internal/server/handlers/info.go
sed -i "s|aerospike/aerospike-backup-service:$PREV_IMAGE_TAG|aerospike/aerospike-backup-service:$NEXT_IMAGE_TAG|" /local/build/docker-compose/docker-compose.yaml
EOF

make -C "$WORKSPACE" docs
