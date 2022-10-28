package report

var TextTemplate = `
Report {{ .Course }} / {{ .Assignment}}

{{range .Projects -}}
{{ .Name}}: {{if .IsActive -}} {{ .WebURL}} {{- else}} --- no activity found --- {{- end}}
{{end}}

`
