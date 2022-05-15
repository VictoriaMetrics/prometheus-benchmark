{{/*
Create a default fully qualified app name.
*/}}
{{- define "prometheus-benchmark.fullname" -}}
{{ .Release.Name }}-{{ .Chart.Name }}
{{- end }}

{{/*
Selector labels for deployed objects.
*/}}
{{- define "prometheus-benchmark.selectorLabels" -}}
chart-name: {{ include "prometheus-benchmark.fullname" . }}
{{- end }}

{{/*
Common labels for all the deployed objects.
They include selector labels plus the recommended labels for Helm charts.
See https://helm.sh/docs/chart_best_practices/labels/
*/}}
{{- define "prometheus-benchmark.labels" -}}
{{ include "prometheus-benchmark.selectorLabels" . }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/version: {{ .Chart.AppVersion }}
{{- end }}
