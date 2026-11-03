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
backupServiceConfig.service.<listener> as YAML, or an empty string when any level is missing,
null or not a map (for example `http:` left with every child commented out). A present block
renders as at least "{}", so the result also tells whether the block exists. Call with
(list $ "http"); "| fromYaml" turns either result into a map.
*/}}
{{- define "aerospike-backup-service.listener" -}}
{{- $cfg := (index . 0).Values.backupServiceConfig -}}
{{- if kindIs "map" $cfg -}}
{{- $svc := index $cfg "service" -}}
{{- if kindIs "map" $svc -}}
{{- $l := index $svc (index . 1) -}}
{{- if kindIs "map" $l }}{{ toYaml $l }}{{ end -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
"true" when ABS serves HTTP: backupServiceConfig.service.http is absent or not disabled
(the same rule as pkg/dto/service_config.go). Empty string otherwise.
*/}}
{{- define "aerospike-backup-service.httpEnabled" -}}
{{- $http := include "aerospike-backup-service.listener" (list . "http") | fromYaml -}}
{{- if not $http.disabled }}true{{ end -}}
{{- end -}}

{{/*
"true" when ABS serves HTTPS: the backupServiceConfig.service.https block is a map and is not
disabled (the same rule as pkg/dto/service_config.go). Empty string otherwise.
*/}}
{{- define "aerospike-backup-service.httpsEnabled" -}}
{{- $https := include "aerospike-backup-service.listener" (list . "https") -}}
{{- if and $https (not ($https | fromYaml).disabled) }}true{{ end -}}
{{- end -}}

{{/*
Port ABS listens on for HTTP: backupServiceConfig.service.http.port, default 8080.
*/}}
{{- define "aerospike-backup-service.httpPort" -}}
{{- $http := include "aerospike-backup-service.listener" (list . "http") | fromYaml -}}
{{- $http.port | default 8080 | int -}}
{{- end -}}

{{/*
Port ABS listens on for HTTPS: backupServiceConfig.service.https.port, default 8443.
*/}}
{{- define "aerospike-backup-service.httpsPort" -}}
{{- $https := include "aerospike-backup-service.listener" (list . "https") | fromYaml -}}
{{- $https.port | default 8443 | int -}}
{{- end -}}

{{/*
The listener the probes, the Ingress, the chart notes and `helm test` use: "http" while HTTP is
enabled, "https" when backupServiceConfig.service.http.disabled is true. It is also the name of
the container and Service port.
*/}}
{{- define "aerospike-backup-service.primaryListener" -}}
{{- if include "aerospike-backup-service.httpEnabled" . }}http{{ else }}https{{ end -}}
{{- end -}}

{{/*
The Service port of the primary listener: service.port for HTTP, service.httpsPort (default the
HTTPS container port) for HTTPS.
*/}}
{{- define "aerospike-backup-service.primaryServicePort" -}}
{{- if include "aerospike-backup-service.httpEnabled" . -}}
{{- .Values.service.port -}}
{{- else -}}
{{- .Values.service.httpsPort | default (include "aerospike-backup-service.httpsPort" .) -}}
{{- end -}}
{{- end -}}

{{/*
The primary listener's context path bounded by slashes ("/" by default), matching
normalizeContextPath in internal/server/server.go, so endpoint names can be appended directly.
*/}}
{{- define "aerospike-backup-service.contextPath" -}}
{{- $l := include "aerospike-backup-service.listener" (list . (include "aerospike-backup-service.primaryListener" .)) | fromYaml -}}
{{- $p := index $l "context-path" | default "/" | toString -}}
{{- if not (hasPrefix "/" $p) }}{{ $p = printf "/%s" $p }}{{ end -}}
{{- if not (hasSuffix "/" $p) }}{{ $p = printf "%s/" $p }}{{ end -}}
{{- $p -}}
{{- end -}}

{{/*
A probe from values, with an httpGet on the primary listener's port and
<context-path><endpoint> added when the values define no handler of their own (scheme HTTPS
when HTTP is disabled). Call with (list $ .Values.livenessProbe "health").
*/}}
{{- define "aerospike-backup-service.probe" -}}
{{- $root := index . 0 -}}
{{- $probe := deepCopy (index . 1 | default dict) -}}
{{- if not (or $probe.httpGet $probe.tcpSocket $probe.exec $probe.grpc) -}}
{{- $listener := include "aerospike-backup-service.primaryListener" $root -}}
{{- $get := dict "path" (printf "%s%s" (include "aerospike-backup-service.contextPath" $root) (index . 2)) "port" $listener -}}
{{- if eq $listener "https" }}{{ $_ := set $get "scheme" "HTTPS" }}{{ end -}}
{{- $_ := set $probe "httpGet" $get -}}
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
Fails the render when the values ask for more than one ABS pod, or disable both listeners.
Each pod needs its own configuration: pods of one release share backupServiceConfig and every
pod runs every routine, so a second pod duplicates every backup. Run a separate release per set
of routines instead.
*/}}
{{- define "aerospike-backup-service.validate" -}}
{{- if gt (int .Values.replicaCount) 1 -}}
{{- fail (printf "replicaCount must be 0 or 1, got %v: each Aerospike Backup Service pod needs its own configuration, and pods of one release share backupServiceConfig. Install a separate release for each set of routines." .Values.replicaCount) -}}
{{- end -}}
{{- $autoscaling := .Values.autoscaling -}}
{{- if or (and (kindIs "map" $autoscaling) $autoscaling.enabled) (and $autoscaling (not (kindIs "map" $autoscaling))) -}}
{{- fail "autoscaling is not supported: each Aerospike Backup Service pod needs its own configuration, so the chart runs exactly one pod per release. Remove autoscaling from your values and install a separate release for each set of routines." -}}
{{- end -}}
{{- if not (or (include "aerospike-backup-service.httpEnabled" .) (include "aerospike-backup-service.httpsEnabled" .)) -}}
{{- fail "backupServiceConfig.service: http and https cannot both be disabled." -}}
{{- end -}}
{{- end -}}
