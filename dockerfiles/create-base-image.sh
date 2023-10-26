#!/bin/bash
docker buildx create --use --name mybuilder
docker login aerospike.jfrog.io -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD"
docker buildx build -t aerospike.jfrog.io/ecosystem-container-dev-local/abs-base-image:latest --push --platform linux/amd64,linux/arm64 \
 --file ./dockerfiles/Dockerfile-base-image .
docker buildx rm mybuilder
