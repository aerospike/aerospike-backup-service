#!/bin/bash -ex
WORKSPACE="$(git rev-parse --show-toplevel)"
CHANNEL="dev"
REGISTRY="docker.io"
REPO="aerospike/aerospike-backup-service"
TAG_LATEST=false
TAG=""
CACHE_TO=""
CACHE_FROM=""
OUTPUT="type=image,push=true"
PLATFORMS="linux/amd64,linux/arm64"

POSITIONAL_ARGS=()

while [[ $# -gt 0 ]]; do
  case $1 in
  --repo)
    REPO="$2"
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
  --registry)
    REGISTRY="$2"
    shift
    shift
    ;;
  --cache-to)
    CACHE_TO="$2"
    shift
    shift
    ;;
  --cache-from)
    CACHE_FROM="$2"
    shift
    shift
    ;;
  --output)
    OUTPUT="$2"
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


docker login aerospike.jfrog.io -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD"


GO_VERSION="$( curl -s 'https://go.dev/dl/?mode=json&include=all' | \
jq -r --arg ver "go$(go mod edit -json | \
jq -r '.Go' | cut -d. -f1-2)" '.[] | select(.version | startswith($ver)) | .version' | \
sort -V | tail -n1 | cut -c3- | tr -d '\n')"

PLATFORMS="$PLATFORMS" \
TAG="$TAG" \
REPO="$REPO" \
CACHE_TO="$CACHE_TO" \
CACHE_FROM="$CACHE_FROM" \
OUTPUT="$OUTPUT" \
REGISTRY="$REGISTRY" \
GOPROXY="$GOPROXY" \
LATEST="$TAG_LATEST" \
GIT_BRANCH="$(git rev-parse --abbrev-ref HEAD)" \
GIT_COMMIT_SHA="$(git rev-parse HEAD)" \
VERSION="$(cat "$WORKSPACE/VERSION")" \
GO_VERSION="$GO_VERSION" \
ISO8601="$(LC_TIME=en_US.UTF-8 date "+%Y-%m-%dT%H:%M:%S%z")" \
CONTEXT="$WORKSPACE" \
docker buildx bake \
--allow=fs.read="$WORKSPACE" \
default \
--progress plain \
--file "$WORKSPACE/build/docker-build/docker-bake.hcl"
