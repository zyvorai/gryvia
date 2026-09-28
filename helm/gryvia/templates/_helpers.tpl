{{/*
Expand the name of the chart.
*/}}
{{- define "gryvia.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "gryvia.fullname" -}}
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
{{- define "gryvia.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "gryvia.labels" -}}
helm.sh/chart: {{ include "gryvia.chart" . }}
{{ include "gryvia.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "gryvia.selectorLabels" -}}
app.kubernetes.io/name: {{ include "gryvia.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "gryvia.serviceAccountName" -}}
{{- if .Values.serviceAccounts.create }}
{{- default (include "gryvia.fullname" .) .Values.serviceAccounts.name }}
{{- else }}
{{- default "default" .Values.serviceAccounts.name }}
{{- end }}
{{- end }}

{{/*
GPU Operator labels
*/}}
{{- define "gryvia.gpuOperator.labels" -}}
{{ include "gryvia.labels" . }}
app.kubernetes.io/component: gpu-operator
{{- end }}

{{/*
AI Operator labels
*/}}
{{- define "gryvia.ioOperator.labels" -}}
{{ include "gryvia.labels" . }}
app.kubernetes.io/component: ai-operator
{{- end }}

{{/*
Storage Operator labels
*/}}
{{- define "gryvia.storageOperator.labels" -}}
{{ include "gryvia.labels" . }}
app.kubernetes.io/component: storage-operator
{{- end }}

{{/*
Network Operator labels
*/}}
{{- define "gryvia.networkOperator.labels" -}}
{{ include "gryvia.labels" . }}
app.kubernetes.io/component: network-operator
{{- end }}

{{/*
Quota Operator labels
*/}}
{{- define "gryvia.quotaOperator.labels" -}}
{{ include "gryvia.labels" . }}
app.kubernetes.io/component: quota-operator
{{- end }}

{{/*
Web UI labels
*/}}
{{- define "gryvia.webUI.labels" -}}
{{ include "gryvia.labels" . }}
app.kubernetes.io/component: web-ui
{{- end }}

{{/*
API Gateway labels
*/}}
{{- define "gryvia.apiGateway.labels" -}}
{{ include "gryvia.labels" . }}
app.kubernetes.io/component: api-gateway
{{- end }}

{{/*
Image pull secrets
*/}}
{{- define "gryvia.imagePullSecrets" -}}
{{- if .Values.global.imagePullSecrets }}
imagePullSecrets:
{{- range .Values.global.imagePullSecrets }}
  - name: {{ . }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Return the appropriate apiVersion for RBAC
*/}}
{{- define "gryvia.rbac.apiVersion" -}}
{{- print "rbac.authorization.k8s.io/v1" }}
{{- end }}

{{/*
Return the appropriate apiVersion for NetworkPolicy
*/}}
{{- define "gryvia.networkPolicy.apiVersion" -}}
{{- print "networking.k8s.io/v1" }}
{{- end }}
