#!/bin/bash -e
WORKSPACE="$(git rev-parse --show-toplevel)"

docker run --rm --volume "$WORKSPACE":/local davi17g/swag:latest init \
-d /local/internal/server,/local/pkg/model -g server.go -o /local/docs

# swag codegen cannot handle int64 format for return values

cat <<< "$(docker run --rm --volume "$WORKSPACE/docs/swagger.json":/local/docs/swagger.json ghcr.io/jqlang/jq:latest \
'(.paths.[].[].responses
| select(has("202")).["202"]
| select(has("schema")).["schema"]
| select(.type | contains("int64"))) = {type: "integer", format: "int64"}' \
/local/docs/swagger.json)" > "$WORKSPACE"/docs/swagger.json

cat <<< "$(docker run --rm --volume "$WORKSPACE/docs/swagger.yaml":/local/docs/swagger.yaml mikefarah/yq:latest \
'(.paths.[].[].responses
| select(has("202")).["202"]
| select(has("schema")).["schema"]
| select(.type | contains("int64"))) = {"type": "integer", "format": "int64"}' \
/local/docs/swagger.yaml)" > "$WORKSPACE"/docs/swagger.yaml

mkdir -p "$WORKSPACE"/tmp

docker run --rm --volume "$WORKSPACE":/local openapitools/openapi-generator-cli:latest generate \
-i /local/docs/swagger.yaml -g openapi-yaml -o /local/tmp

docker run --rm --volume "$WORKSPACE":/local openapitools/openapi-generator-cli:latest generate \
-i /local/docs/swagger.json -g openapi -o /local/tmp

mv "$WORKSPACE"/tmp/openapi/openapi.yaml "$WORKSPACE"/docs/openapi.yaml
mv "$WORKSPACE"/tmp/openapi.json "$WORKSPACE"/docs/openapi.json

rm -rf "$WORKSPACE"/tmp "$WORKSPACE"/docs/swagger.yaml "$WORKSPACE"/docs/swagger.json
