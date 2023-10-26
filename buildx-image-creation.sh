#!/bin/bash
TAG_LATEST=${TAG_LATEST:-false}
LATEST_STRING=""
if [ $TAG_LATEST = true ]; then
    LATEST_STRING="-t aerospike.jfrog.io/ecosystem-container-dev-local/aerospike-backup-service:latest"
fi

docker buildx create --use --name mybuilder
docker login aerospike.jfrog.io -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD"
docker buildx build -t aerospike.jfrog.io/ecosystem-container-dev-local/aerospike-backup-service:"$TAG" $LATEST_STRING --push --platform linux/amd64,linux/arm64 \
--build-arg BASE_IMAGE=aerospike.jfrog.io/ecosystem-container-dev-local/abs-base-image:latest .
docker buildx rm mybuilder
