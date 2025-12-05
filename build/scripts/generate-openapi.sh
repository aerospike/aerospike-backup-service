#!/bin/bash -e
WORKSPACE="$(git rev-parse --show-toplevel)"

# generate swagger documentation using https://github.com/swaggo/swag tool
docker run --rm --volume "$WORKSPACE":/local davi17g/swag:latest init \
-d /local/internal/server/handlers,/local/pkg/dto -g info.go -o /local/docs

# swag codegen cannot handle int64 format for return values
cat <<< "$(docker run --rm --volume "$WORKSPACE/docs/swagger.json":/local/docs/swagger.json ghcr.io/jqlang/jq:latest \
'(.paths.[].[].responses
| select(has("202")).["202"]
| select(has("schema")).["schema"]
| select(.type | contains("int64"))) = {type: "integer", format: "int64"}' \
/local/docs/swagger.json)" > "$WORKSPACE"/docs/swagger.json

mkdir -p "$WORKSPACE"/tmp

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

openapi-generator generate -i "$WORKSPACE"/docs/openapi.json -g typescript-fetch -o "$WORKSPACE"/frontend/src/api/generated --additional-properties=typescriptThreePlus=true