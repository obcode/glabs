package report

var HTMLTemplate = `
<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<title>glabs {{ .Course }} / {{ .Assignment }}</title>
	</head>
	<body>
		<h1>Report <a href="{{ .URL }}">{{ .Course }} / {{ .Assignment }} </a> </h1>
		<h3>{{ .Description }}</h3>
		<ol>
		{{range .Projects -}}
			<li>
				<a href="{{ .WebURL}}">{{ .Name}}</a>
				Last Activity {{ .LastActivity.Format "02.01.06, 15:04 MST"}}
			</li>
		{{end}}
		</ol>
	</body>
</html>
`