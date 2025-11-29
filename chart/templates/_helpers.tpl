{{/*
Expand the name of the chart.
*/}}
{{- define "k8s-event-monitor.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "k8s-event-monitor.fullname" -}}
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
{{- define "k8s-event-monitor.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "k8s-event-monitor.labels" -}}
helm.sh/chart: {{ include "k8s-event-monitor.chart" . }}
{{ include "k8s-event-monitor.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "k8s-event-monitor.selectorLabels" -}}
app.kubernetes.io/name: {{ include "k8s-event-monitor.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "k8s-event-monitor.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "k8s-event-monitor.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Convert a Kind name into the Kubernetes REST resource string. Accepts an object
with optional "resource" override and "kind" fallback.
*/}}
{{- define "k8s-event-monitor.kindToResource" -}}
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
