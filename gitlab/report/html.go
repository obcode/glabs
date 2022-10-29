package report

var HTMLTemplate = `
<!DOCTYPE html>
<html data-theme="cupcake">
	<head>
		<meta charset="UTF-8">
		<title>glabs {{ .Course }} / {{ .Assignment }}</title>
		<link href="https://cdn.jsdelivr.net/npm/daisyui@2.33.0/dist/full.css" rel="stylesheet" type="text/css" />
		<script src="https://cdn.tailwindcss.com"></script>
	</head>
	<body>
	<div class="p-8">
	<div class="text-center m-2">
		<div class="text-4xl text-center mt-8">
			Report <a href="{{ .URL }}">{{ .Course }} / {{ .Assignment }} </a>
		</div>
		<div class="text-xl text-center m-8">{{ .Description }}</div>
		<div class="text-xl text-center m-8">Generated: {{ .Generated.Format "02.01.06, 15:04 MST" }}</div>
	</div>
		<table class="table table-compact w-full">
		<thead>
		<tr>
			<th>Name</th> 
			<th>Member</th>
			<th>Last Commit</th>
			<th>Open Issues</th>
			<th>Open Merge Requests</th>
			{{if .HasReleaseMergeRequest -}}
				<th>Release.MergeRequest</th>
			{{- end}}
			{{if .HasReleaseDockerImages -}}
				<th>Release.DockerImages</th>
			{{- end}}
		</tr>
		</thead> 
		<tbody>
		{{range .Projects -}}
			{{if and .IsActive (gt .Commits 0) }}<tr class="active">
			{{else}} <tr>
			{{end}}
			<td><a href="{{ .WebURL}}">{{ .Name}}</a></td>
			<td>
				{{range .Members}}
					<a href="{{ .WebURL }}">{{ .Name}}</a>,
				{{end}}
			</td>
			<td>
				{{if .LastCommit -}}
				<a href="{{ .LastCommit.WebURL}}">last commit</a>
				{{- end}}
			</td>
			<td>
				{{if eq .OpenIssuesCount 1 -}}
				<a href="{{ .WebURL }}/-/issues">{{ .OpenIssuesCount }} open issue</a>
				{{else}}
				{{if gt .OpenIssuesCount 1 -}}
			   	<a href="{{ .WebURL }}/-/issues">{{ .OpenIssuesCount }} open issues</a>
				{{- end}}
				{{- end}}
			</td>
			<td>
				{{if gt .OpenMergeRequestsCount 0 -}}
				<a href="{{ .WebURL }}/-/merge_requests">{{ .OpenMergeRequestsCount }} open merge requests</a>
				{{- end}}
			</td>
			{{if .Release -}}
				{{if .Release.MergeRequest -}}
					{{if .Release.MergeRequest.Found -}}
						<td>
							<a href="{{ .Release.MergeRequest.WebURL }}">
								merge request ({{ .Release.MergeRequest.PipelineStatus}})
							</a>
						</td>
					{{- else}}
						<td></td>
					{{- end}}
				{{- end}}
				{{if .Release.DockerImages -}}
					<td>
					<a href="{{ .WebURL }}/container_registry">
						{{ .Release.DockerImages.Status }}
					</a>
					</td>
				{{- end}}
			{{- end}}
			</tr>
			{{end}}
		</tbody>
		</table>
	</div>

	<footer class="footer footer-center p-4 bg-base-300 text-base-content">
	<div>
		<p>Generated with <a href="https://github.com/obcode/glabs">glabs</a>, {{ .Generated.Format "02.01.06, 15:04 MST" }}</p>
	</div>
	</footer>
	</body>
</html>
`
