{{/*
Expand the name of the chart.
*/}}
{{- define "aerospike-backup-service.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "aerospike-backup-service.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "aerospike-backup-service.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "aerospike-backup-service.labels" -}}
helm.sh/chart: {{ include "aerospike-backup-service.chart" . }}
{{ include "aerospike-backup-service.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "aerospike-backup-service.selectorLabels" -}}
app.kubernetes.io/name: {{ include "aerospike-backup-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "aerospike-backup-service.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "aerospike-backup-service.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{- define "args.list" -}}
{{- if empty $.Values.args }}
{{- $list := list "-c" "/etc/aerospike-backup-service/aerospike-backup-service.yml" -}}
{{- $list | toYaml }}
{{- else }}
{{- $.Values.args | toYaml }}
{{- end -}}
{{- end -}}

{{/*
backupServiceConfig.service.<listener> as YAML, or {} when any level is missing, null or not
a map (for example `http:` left with every child commented out). Call with (list $ "http").
Usage: include "aerospike-backup-service.listener" (list $ "http") | fromYaml
*/}}
{{- define "aerospike-backup-service.listener" -}}
{{- $root := index . 0 -}}
{{- $out := dict -}}
{{- $cfg := $root.Values.backupServiceConfig -}}
{{- if kindIs "map" $cfg -}}
{{- $svc := index $cfg "service" -}}
{{- if kindIs "map" $svc -}}
{{- $l := index $svc (index . 1) -}}
{{- if kindIs "map" $l }}{{ $out = $l }}{{ end -}}
{{- end -}}
{{- end -}}
{{- toYaml $out -}}
{{- end -}}

{{/*
Port ABS listens on for HTTP: backupServiceConfig.service.http.port, default 8080.
*/}}
{{- define "aerospike-backup-service.httpPort" -}}
{{- $http := include "aerospike-backup-service.listener" (list . "http") | fromYaml -}}
{{- $http.port | default 8080 | int -}}
{{- end -}}

{{/*
"true" when ABS serves HTTPS: the backupServiceConfig.service.https block is a map and is not
disabled (the same rule as pkg/dto/service_config.go). Empty string otherwise.
*/}}
{{- define "aerospike-backup-service.httpsEnabled" -}}
{{- $cfg := .Values.backupServiceConfig -}}
{{- if and (kindIs "map" $cfg) (kindIs "map" (index $cfg "service")) (kindIs "map" (index (index $cfg "service") "https")) -}}
{{- $https := include "aerospike-backup-service.listener" (list . "https") | fromYaml -}}
{{- if not $https.disabled }}true{{ end -}}
{{- end -}}
{{- end -}}

{{/*
Port ABS listens on for HTTPS: backupServiceConfig.service.https.port, default 8443.
*/}}
{{- define "aerospike-backup-service.httpsPort" -}}
{{- $https := include "aerospike-backup-service.listener" (list . "https") | fromYaml -}}
{{- $https.port | default 8443 | int -}}
{{- end -}}

{{/*
The HTTP listener's context path bounded by slashes ("/" by default), matching
normalizeContextPath in internal/server/server.go, so endpoint names can be appended directly.
*/}}
{{- define "aerospike-backup-service.contextPath" -}}
{{- $http := include "aerospike-backup-service.listener" (list . "http") | fromYaml -}}
{{- $p := index $http "context-path" | default "/" | toString -}}
{{- if not (hasPrefix "/" $p) }}{{ $p = printf "/%s" $p }}{{ end -}}
{{- if not (hasSuffix "/" $p) }}{{ $p = printf "%s/" $p }}{{ end -}}
{{- $p -}}
{{- end -}}

{{/*
A probe from values, with an httpGet on the named "http" port and <context-path><endpoint>
added when the values define no handler of their own. Call with (list $ .Values.livenessProbe "health").
*/}}
{{- define "aerospike-backup-service.probe" -}}
{{- $root := index . 0 -}}
{{- $probe := deepCopy (index . 1 | default dict) -}}
{{- if not (or $probe.httpGet $probe.tcpSocket $probe.exec $probe.grpc) -}}
{{- $_ := set $probe "httpGet" (dict "path" (printf "%s%s" (include "aerospike-backup-service.contextPath" $root) (index . 2)) "port" "http") -}}
{{- end -}}
{{- toYaml $probe -}}
{{- end -}}

{{/*
Resolve a volume mount's readOnly flag.
Accepts a YAML boolean, or the strings "true"/"false" (Terraform and other templating
layers serialise booleans as strings). Anything else, including an absent value,
defaults to true so a mount is never made writable by accident.
*/}}
{{- define "aerospike-backup-service.readOnly" -}}
{{- if kindIs "bool" . }}{{ . }}
{{- else if eq (lower (toString .)) "false" }}false
{{- else }}true
{{- end }}
{{- end -}}

{{/*
Fails the render when the values ask for more than one ABS pod. Each pod needs its own
configuration: pods of one release share backupServiceConfig and every pod runs every routine,
so a second pod duplicates every backup. Run a separate release per set of routines instead.
*/}}
{{- define "aerospike-backup-service.validate" -}}
{{- if gt (int .Values.replicaCount) 1 -}}
{{- fail (printf "replicaCount must be 0 or 1, got %v: each Aerospike Backup Service pod needs its own configuration, and pods of one release share backupServiceConfig. Install a separate release for each set of routines." .Values.replicaCount) -}}
{{- end -}}
{{- if dig "enabled" false (.Values.autoscaling | default dict) -}}
{{- fail "autoscaling is not supported: each Aerospike Backup Service pod needs its own configuration, so the chart runs exactly one pod per release. Remove autoscaling from your values and install a separate release for each set of routines." -}}
{{- end -}}
{{- end -}}
