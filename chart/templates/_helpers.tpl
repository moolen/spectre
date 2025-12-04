{{/*
Expand the name of the chart.
*/}}
{{- define "spectre.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "spectre.fullname" -}}
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
{{- define "spectre.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "spectre.labels" -}}
helm.sh/chart: {{ include "spectre.chart" . }}
{{ include "spectre.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "spectre.selectorLabels" -}}
app.kubernetes.io/name: {{ include "spectre.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "spectre.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "spectre.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Convert a Kind name into the Kubernetes REST resource string. Accepts an object
with optional "resource" override and "kind" fallback.
*/}}
{{- define "spectre.kindToResource" -}}
{{- $override := default "" .resource -}}
{{- if $override }}
{{- lower $override -}}
{{- else }}
{{- $kind := lower (default "" .kind) -}}
{{- if eq $kind "" -}}
{{- "" -}}
{{- else if eq $kind "ingress" -}}
ingresses
{{- else if hasSuffix $kind "s" -}}
{{ $kind }}
{{- else -}}
{{ printf "%ss" $kind }}
{{- end -}}
{{- end -}}
{{- end }}
