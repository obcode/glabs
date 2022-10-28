package report

var TextTemplate = `
Report Projects
===============

{{range .Projects -}}
{{ .Name}}: Last Activity {{ .LastActivity.Format "02.01.06, 15:04 MST"}}
{{end}}

`
