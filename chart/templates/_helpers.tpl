{{/*
Create a default fully qualified app name.
*/}}
{{- define "prometheus-benchmark.fullname" -}}
{{- $nameParts := list .Release.Name .Chart.Name }}
{{- if hasKey . "component" }}
{{- $nameParts = append $nameParts .component }}
{{- end }}
{{- if hasKey . "type" }}
{{- $nameParts = append $nameParts .type}}
{{- end }}
{{- if hasKey . "storage" }}
{{- $nameParts = append $nameParts .storage }}
{{- end }}
{{- join "-" $nameParts }}
{{- end }}

{{/*
Selector labels for deployed objects.
*/}}
{{- define "prometheus-benchmark.selectorLabels" -}}
chart-name: {{ include "prometheus-benchmark.fullname" . }}
{{- if hasKey . "component" }}
component: {{ .component }}
{{- end }}
{{- if hasKey . "type" }}
type: {{ .type }}
{{- end }}
{{- if hasKey . "storage" }}
storage: {{ .storage }}
{{- end }}
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
