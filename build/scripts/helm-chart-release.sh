#!/bin/bash -e
NEXT_ELM_CHART_VERSION="$1"
WORKSPACE="$(git rev-parse --show-toplevel)"
APP_VERSION="$(cat "$WORKSPACE"/VERSION | cut -c 2-)"

POSITIONAL_ARGS=()

yq -i --unwrapScalar=false ".version = \"${NEXT_ELM_CHART_VERSION}\"" "$WORKSPACE/helm/aerospike-backup-service/Chart.yaml"
yq -i --unwrapScalar=false ".appVersion = \"${APP_VERSION}\"" "$WORKSPACE/helm/aerospike-backup-service/Chart.yaml"
yq -i --unwrapScalar=false ".image.tag = \"${APP_VERSION}\"" "$WORKSPACE/helm/aerospike-backup-service/values.yaml"
