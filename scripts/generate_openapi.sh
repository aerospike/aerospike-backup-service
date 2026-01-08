#!/bin/bash

# Generate the OpenAPI server and DTOs
# Outputting to internal/server/openapi

rm -rf internal/server/openapi
mkdir -p internal/server/openapi

openapi-generator generate \
    -i docs/openapi_v2.json \
    -g go-server \
    -o internal/server/openapi \
    --additional-properties=packageName=openapi,router=chi,sourceFolder=.,serverPort=8080,featureCORS=true,outputAsLibrary=true \
    --global-property=models,apis,supportingFiles \
    --git-host=github.com \
    --git-user-id=aerospike \
    --git-repo-id=aerospike-backup-service/v3/internal/server/openapi

# Clean up unnecessary files
 rm internal/server/openapi/README.md