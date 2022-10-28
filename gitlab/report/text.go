package report

var TextTemplate = `
Report {{ .Course }} / {{ .Assignment}}

{{range .Projects -}}
{{ .Name -}}:
{{- if .IsActive -}}
	{{- if eq .Commits 0 }} --- no commits found ---
	{{- else }} {{ .WebURL -}}
	{{- end -}}
{{- else }} --- no activity found ---
{{- end}}
{{end}}

`
