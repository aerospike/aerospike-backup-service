#!/bin/bash -e
WORKSPACE="$(git rev-parse --show-toplevel)"
DOCKER_USER=(--user "$(id -u):$(id -g)")

# generate swagger documentation using https://github.com/swaggo/swag tool
docker run --rm "${DOCKER_USER[@]}" --volume "$WORKSPACE":/local davi17g/swag:latest init \
-d /local/internal/server/handlers,/local/pkg/dto -g info.go -o /local/docs

mkdir -p "$WORKSPACE"/tmp

# swag codegen cannot handle int64 format for return values
docker run --rm --volume "$WORKSPACE/docs":/local/docs:ro ghcr.io/jqlang/jq:latest \
'(.paths.[].[].responses
| select(has("202")).["202"]
| select(has("schema")).["schema"]
| select(.type | contains("int64"))) = {type: "integer", format: "int64"}' \
/local/docs/swagger.json > "$WORKSPACE"/tmp/swagger.json
mv "$WORKSPACE"/tmp/swagger.json "$WORKSPACE"/docs/swagger.json

# convert swagger to open-api using swagger2openapi
yes | npx swagger2openapi "$WORKSPACE"/docs/swagger.json -o "$WORKSPACE"/docs/openapi.json

rm -rf "$WORKSPACE"/tmp "$WORKSPACE"/docs/swagger.yaml "$WORKSPACE"/docs/swagger.json

# generate config schema
jq '{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": .components.schemas["dto.Config"].properties,
  "components": {
    "schemas": ( .components.schemas | del(.["dto.Config"]) )
  }
}' "$WORKSPACE"/docs/openapi.json > "$WORKSPACE"/docs/config.schema.json
