#!/bin/bash -e
WORKSPACE="$(git rev-parse --show-toplevel)"
CHANNEL="dev"
TAG_LATEST=false
TAG=""
PLATFORMS="linux/amd64,linux/arm64"


POSITIONAL_ARGS=()

while [[ $# -gt 0 ]]; do
  case $1 in
  --channel)
    CHANNEL="$2"
    shift
    shift
    ;;
  --tag)
    TAG="$2"
    shift
    shift
    ;;
  --tag-latest)
    TAG_LATEST="$2"
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


if [ "$CHANNEL" == "dev" ]; then
  HUB="aerospike.jfrog.io/ecosystem-container-dev-local"
elif [ "$CHANNEL" == "stage" ]; then
  HUB="aerospike.jfrog.io/ecosystem-container-stage-local"
elif [ "$CHANNEL" == "prod" ]; then
  HUB="aerospike.jfrog.io/ecosystem-container-prod-local"
else
  echo "Unknown channel"
  exit 1
fi

docker login aerospike.jfrog.io -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD"
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
docker buildx rm builder || true

docker buildx create --name builder --driver docker-container --use
docker buildx inspect --bootstrap

PLATFORMS="$PLATFORMS" \
TAG="$TAG" \
HUB="$HUB" \
LATEST="$TAG_LATEST" \
GIT_BRANCH="$(git rev-parse --abbrev-ref HEAD)" \
GIT_COMMIT_SHA="$(git rev-parse HEAD)" \
VERSION="$(cat "$WORKSPACE/VERSION")" \
ISO8601="$(LC_TIME=en_US.UTF-8 date "+%Y-%m-%dT%H:%M:%S%z")" \
CONTEXT="$WORKSPACE" \
docker buildx bake default \
--progress plain \
--file "$WORKSPACE/build/docker-build/docker-bake.hcl"

docker context use default
docker buildx rm builder
