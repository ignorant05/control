{{- define "control-api.fullname" -}}
{{ .Release.Name }}-control-api
{{- end -}}

{{- define "control-api.labels" -}}
app.kubernetes.io/name: control-api
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
