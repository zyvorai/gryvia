{{/*
Expand the name of the chart.
*/}}
{{- define "tensorreaper.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "tensorreaper.fullname" -}}
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
{{- define "tensorreaper.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "tensorreaper.labels" -}}
helm.sh/chart: {{ include "tensorreaper.chart" . }}
{{ include "tensorreaper.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "tensorreaper.selectorLabels" -}}
app.kubernetes.io/name: {{ include "tensorreaper.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "tensorreaper.serviceAccountName" -}}
{{- if .Values.serviceAccounts.create }}
{{- default (include "tensorreaper.fullname" .) .Values.serviceAccounts.name }}
{{- else }}
{{- default "default" .Values.serviceAccounts.name }}
{{- end }}
{{- end }}

{{/*
GPU Operator labels
*/}}
{{- define "tensorreaper.gpuOperator.labels" -}}
{{ include "tensorreaper.labels" . }}
app.kubernetes.io/component: gpu-operator
{{- end }}

{{/*
AI Operator labels
*/}}
{{- define "tensorreaper.aiOperator.labels" -}}
{{ include "tensorreaper.labels" . }}
app.kubernetes.io/component: ai-operator
{{- end }}

{{/*
Storage Operator labels
*/}}
{{- define "tensorreaper.storageOperator.labels" -}}
{{ include "tensorreaper.labels" . }}
app.kubernetes.io/component: storage-operator
{{- end }}

{{/*
Network Operator labels
*/}}
{{- define "tensorreaper.networkOperator.labels" -}}
{{ include "tensorreaper.labels" . }}
app.kubernetes.io/component: network-operator
{{- end }}

{{/*
Quota Operator labels
*/}}
{{- define "tensorreaper.quotaOperator.labels" -}}
{{ include "tensorreaper.labels" . }}
app.kubernetes.io/component: quota-operator
{{- end }}

{{/*
Web UI labels
*/}}
{{- define "tensorreaper.webUI.labels" -}}
{{ include "tensorreaper.labels" . }}
app.kubernetes.io/component: web-ui
{{- end }}

{{/*
API Gateway labels
*/}}
{{- define "tensorreaper.apiGateway.labels" -}}
{{ include "tensorreaper.labels" . }}
app.kubernetes.io/component: api-gateway
{{- end }}

{{/*
Image pull secrets
*/}}
{{- define "tensorreaper.imagePullSecrets" -}}
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
{{- define "tensorreaper.rbac.apiVersion" -}}
{{- print "rbac.authorization.k8s.io/v1" }}
{{- end }}

{{/*
Return the appropriate apiVersion for NetworkPolicy
*/}}
{{- define "tensorreaper.networkPolicy.apiVersion" -}}
{{- print "networking.k8s.io/v1" }}
{{- end }}
