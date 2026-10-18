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
# this for final releases; checking here too keeps the failure local to the machine cutting the
# release. One-off test tags (vX.Y.Z-suffix) pick their own chart version and skip this rule.
#
# The rule only applies from CUTOVER onward, matching pre-release.yml's own guard. Lines below it
# predate the convention and already carry chart versions that cannot satisfy it: the 3.6 line
# sits on chart 2.0.11, so demanding a chart ending in .3 for a v3.6.3 hotfix would force either
# 2.0.3 (sorts below an already-published chart, which pre-release.yml then rejects) or a
# needlessly high minor that would in turn block the next release's chart.
CUTOVER="3.7.0"

# True when $1 sorts strictly below $2 -- same comparison pre-release.yml uses.
lower() {
  [ "$1" != "$2" ] && [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)" = "$1" ]
}

if echo "$APP_VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$' && ! lower "$APP_VERSION" "$CUTOVER"; then
  APP_PATCH="${APP_VERSION##*.}"
  CHART_PATCH="${NEXT_HELM_CHART_VERSION##*.}"
  if [ "$CHART_PATCH" != "$APP_PATCH" ]; then
    echo "helm-chart-release: app $APP_VERSION needs a chart version ending in .$APP_PATCH," >&2
    echo "  got $NEXT_HELM_CHART_VERSION. A new minor line bumps the chart minor (3.7.0 -> 2.1.0);" >&2
    echo "  a hotfix bumps the chart patch (3.7.1 -> 2.1.1)." >&2
    exit 1
  fi
fi

yq -i --unwrapScalar=false ".version = \"${NEXT_HELM_CHART_VERSION}\"" "$WORKSPACE/helm/aerospike-backup-service/Chart.yaml"
yq -i --unwrapScalar=false ".appVersion = \"${APP_VERSION}\"" "$WORKSPACE/helm/aerospike-backup-service/Chart.yaml"
yq -i --unwrapScalar=false ".image.tag = \"${APP_VERSION}\"" "$WORKSPACE/helm/aerospike-backup-service/values.yaml"
