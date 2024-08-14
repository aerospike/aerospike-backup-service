#!/bin/bash -e
WORKSPACE="$(git rev-parse --show-toplevel)"
NEXT_VERSION="$1"
PREV_VERSION="$(cat "$WORKSPACE"/VERSION)"

docker run --rm --interactive --volume "$WORKSPACE":/local bash:latest <<EOF
sed -i "s/$PREV_VERSION/$NEXT_VERSION/g" /local/VERSION
sed -i "s/$PREV_VERSION/$NEXT_VERSION/" /local/internal/server/handlers/info.go
EOF

bash -c "$WORKSPACE"/build/scripts/generate-openapi.sh
