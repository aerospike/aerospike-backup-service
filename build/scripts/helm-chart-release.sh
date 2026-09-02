#!/bin/bash -e
NEXT_HELM_CHART_VERSION="$1"

# `make release` lists this script after service-release, so plain `NEXT_VERSION=... make
# release` reaches here with no argument and would stamp `version: ""` into Chart.yaml. That
# only surfaces much later, as an opaque failure in the release pipeline.
if [ -z "$NEXT_HELM_CHART_VERSION" ]; then
  echo "helm-chart-release: NEXT_HELM_CHART_VERSION is required, e.g." >&2
  echo "  NEXT_HELM_CHART_VERSION=2.1.0 make helm-chart-release" >&2
  exit 1
fi

if ! echo "$NEXT_HELM_CHART_VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "helm-chart-release: '$NEXT_HELM_CHART_VERSION' is not a bare X.Y.Z version." >&2
  exit 1
fi

WORKSPACE="$(git rev-parse --show-toplevel)"
APP_VERSION="$(cat "$WORKSPACE"/VERSION | cut -c 2-)"

# The chart minor advances once per app minor line and the chart patch mirrors the app patch,
# so that charts sort in the same order as the app versions they ship. pre-release.yml enforces
# this; checking here too keeps the failure local to the machine cutting the release.
APP_PATCH="${APP_VERSION##*.}"
CHART_PATCH="${NEXT_HELM_CHART_VERSION##*.}"
if [ "$CHART_PATCH" != "$APP_PATCH" ]; then
  echo "helm-chart-release: app $APP_VERSION needs a chart version ending in .$APP_PATCH," >&2
  echo "  got $NEXT_HELM_CHART_VERSION. A new minor line bumps the chart minor (3.7.0 -> 2.1.0);" >&2
  echo "  a hotfix bumps the chart patch (3.7.1 -> 2.1.1)." >&2
  exit 1
fi

yq -i --unwrapScalar=false ".version = \"${NEXT_HELM_CHART_VERSION}\"" "$WORKSPACE/helm/aerospike-backup-service/Chart.yaml"
yq -i --unwrapScalar=false ".appVersion = \"${APP_VERSION}\"" "$WORKSPACE/helm/aerospike-backup-service/Chart.yaml"
yq -i --unwrapScalar=false ".image.tag = \"${APP_VERSION}\"" "$WORKSPACE/helm/aerospike-backup-service/values.yaml"
