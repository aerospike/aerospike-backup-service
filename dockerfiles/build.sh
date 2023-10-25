#!/bin/bash

BUILDER_NAME="aerospike-builder"
TAG_LATEST=false
TAG=""
PLATFORMS="linux/amd64,linux/arm64"
DOCKERFILE="./dockerfiles/Dockerfile"
POSITIONAL_ARGS=()

while [[ $# -gt 0 ]]; do
  case $1 in
  --tag)
    TAG="$2"
    shift
    shift
    ;;
  --tag-latest)
    TAG_LATEST=true
    shift
    ;;
  --platforms)
    PLATFORMS="$2"
    shift
    shift
    ;;
  -* | --*)
    echo "Unknown option $1"
    exit 1
    ;;
  *)
    POSITIONAL_ARGS+=("$1") # save positional arg
    shift                   # past argument
    ;;
  esac
done

set -- "${POSITIONAL_ARGS[@]}"

docker buildx create --name "$BUILDER_NAME" --use

docker login aerospike.jfrog.io -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD"
PLATFORMS="$PLATFORMS" TAG="$TAG" LATEST="$TAG_LATEST" docker buildx bake --no-cache --file docker-bake.hcl \
--file "$DOCKERFILE"

docker buildx rm "$BUILDER_NAME"
