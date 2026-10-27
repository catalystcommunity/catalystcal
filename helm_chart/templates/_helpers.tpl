{{/* Chart name. */}}
{{- define "catalystcal.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/* Fully qualified app name. */}}
{{- define "catalystcal.fullname" -}}
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

{{- define "catalystcal.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{- define "catalystcal.labels" -}}
helm.sh/chart: {{ include "catalystcal.chart" . }}
{{ include "catalystcal.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{- define "catalystcal.selectorLabels" -}}
app.kubernetes.io/name: {{ include "catalystcal.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{- define "catalystcal.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "catalystcal.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/* Name of the Secret with CATALYSTCAL_JWT_SECRET and CATALYSTCAL_LINKKEYS_PKI_API_KEY. */}}
{{- define "catalystcal.secretName" -}}
{{- if .Values.secrets.existingSecret }}
{{- .Values.secrets.existingSecret }}
{{- else }}
{{- include "catalystcal.fullname" . }}
{{- end }}
{{- end }}

{{/* Name of the Secret with the act-as application keys file. */}}
{{- define "catalystcal.actAsSecretName" -}}
{{- if .Values.actAs.existingSecret }}
{{- .Values.actAs.existingSecret }}
{{- else }}
{{- printf "%s-act-as" (include "catalystcal.fullname" .) }}
{{- end }}
{{- end }}

{{/* Name of the API data PersistentVolumeClaim. */}}
{{- define "catalystcal.dataClaimName" -}}
{{- if .Values.api.persistence.existingClaim }}
{{- .Values.api.persistence.existingClaim }}
{{- else }}
{{- printf "%s-data" (include "catalystcal.fullname" .) }}
{{- end }}
{{- end }}

{{/* Name of the backup PersistentVolumeClaim. */}}
{{- define "catalystcal.backupClaimName" -}}
{{- if .Values.backup.persistence.existingClaim }}
{{- .Values.backup.persistence.existingClaim }}
{{- else }}
{{- printf "%s-backup" (include "catalystcal.fullname" .) }}
{{- end }}
{{- end }}

{{/* The callback URL: config.callbackUrl, or <publicUrl>/auth/callback. */}}
{{- define "catalystcal.callbackUrl" -}}
{{- if .Values.config.callbackUrl }}
{{- .Values.config.callbackUrl }}
{{- else if .Values.config.publicUrl }}
{{- printf "%s/auth/callback" (trimSuffix "/" .Values.config.publicUrl) }}
{{- end }}
{{- end }}
