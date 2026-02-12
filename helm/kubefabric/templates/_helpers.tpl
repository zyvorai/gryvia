{{/*
Expand the name of the chart.
*/}}
{{- define "kubefabric.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "kubefabric.fullname" -}}
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
{{- define "kubefabric.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "kubefabric.labels" -}}
helm.sh/chart: {{ include "kubefabric.chart" . }}
{{ include "kubefabric.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "kubefabric.selectorLabels" -}}
app.kubernetes.io/name: {{ include "kubefabric.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "kubefabric.serviceAccountName" -}}
{{- if .Values.serviceAccounts.create }}
{{- default (include "kubefabric.fullname" .) .Values.serviceAccounts.name }}
{{- else }}
{{- default "default" .Values.serviceAccounts.name }}
{{- end }}
{{- end }}

{{/*
GPU Operator labels
*/}}
{{- define "kubefabric.gpuOperator.labels" -}}
{{ include "kubefabric.labels" . }}
app.kubernetes.io/component: gpu-operator
{{- end }}

{{/*
AI Operator labels
*/}}
{{- define "kubefabric.aiOperator.labels" -}}
{{ include "kubefabric.labels" . }}
app.kubernetes.io/component: ai-operator
{{- end }}

{{/*
Storage Operator labels
*/}}
{{- define "kubefabric.storageOperator.labels" -}}
{{ include "kubefabric.labels" . }}
app.kubernetes.io/component: storage-operator
{{- end }}

{{/*
Network Operator labels
*/}}
{{- define "kubefabric.networkOperator.labels" -}}
{{ include "kubefabric.labels" . }}
app.kubernetes.io/component: network-operator
{{- end }}

{{/*
Quota Operator labels
*/}}
{{- define "kubefabric.quotaOperator.labels" -}}
{{ include "kubefabric.labels" . }}
app.kubernetes.io/component: quota-operator
{{- end }}

{{/*
Web UI labels
*/}}
{{- define "kubefabric.webUI.labels" -}}
{{ include "kubefabric.labels" . }}
app.kubernetes.io/component: web-ui
{{- end }}

{{/*
API Gateway labels
*/}}
{{- define "kubefabric.apiGateway.labels" -}}
{{ include "kubefabric.labels" . }}
app.kubernetes.io/component: api-gateway
{{- end }}

{{/*
Image pull secrets
*/}}
{{- define "kubefabric.imagePullSecrets" -}}
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
{{- define "kubefabric.rbac.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "rbac.authorization.k8s.io/v1" }}
{{- print "rbac.authorization.k8s.io/v1" }}
{{- else }}
{{- print "rbac.authorization.k8s.io/v1beta1" }}
{{- end }}
{{- end }}

{{/*
Return the appropriate apiVersion for NetworkPolicy
*/}}
{{- define "kubefabric.networkPolicy.apiVersion" -}}
{{- if .Capabilities.APIVersions.Has "networking.k8s.io/v1" }}
{{- print "networking.k8s.io/v1" }}
{{- else }}
{{- print "networking.k8s.io/v1beta1" }}
{{- end }}
{{- end }}
