package gitlab

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/obcode/glabs/config"
	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

func TestProjectReport_AggregatesReleaseAndCommitData(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/7/repository/branches":
			_, _ = w.Write([]byte(`[{"name":"main"},{"name":"develop"}]`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/7/repository/commits":
			ref := r.URL.Query().Get("ref_name")
			if ref == "main" {
				_, _ = w.Write([]byte(`[
					{"title":"initial","committer_name":"alice","committed_date":"2026-04-20T10:00:00Z","web_url":"https://gitlab.example.org/c1"}
				]`))
				return
			}
			if ref == "develop" {
				_, _ = w.Write([]byte(`[
					{"title":"latest","committer_name":"bob","committed_date":"2026-04-22T12:00:00Z","web_url":"https://gitlab.example.org/c2"}
				]`))
				return
			}
			_, _ = w.Write([]byte(`[]`))
			return

		case r.Method == http.MethodGet &&
			(r.URL.Path == "/api/v4/projects/7/members" || r.URL.Path == "/api/v4/projects/7/members/all"):
			_, _ = w.Write([]byte(`[{"id":100,"name":"Alice","username":"alice","web_url":"https://gitlab.example.org/alice"}]`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/7/merge_requests":
			if r.URL.Query().Get("state") != "opened" {
				t.Fatalf("merge request state query = %q", r.URL.Query().Get("state"))
			}
			_, _ = w.Write([]byte(`[
				{"iid":11,"source_branch":"develop","target_branch":"main","web_url":"https://gitlab.example.org/mr/11"}
			]`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/7/merge_requests/11/pipelines":
			_, _ = w.Write([]byte(`[
				{"id":1,"status":"failed","created_at":"2026-04-22T11:00:00Z"},
				{"id":2,"status":"success","created_at":"2026-04-22T12:30:00Z"}
			]`))
			return

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v4/projects/7/registry/repositories"):
			_, _ = w.Write([]byte(`[
				{"name":"registry/app","location":"registry.example.org/mpd/ss26/repo/app"}
			]`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	createdAt := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	lastActivity := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	project := &gitlabapi.Project{
		ID:              7,
		Name:            "mpd-a1-team1",
		CreatedAt:       &createdAt,
		LastActivityAt:  &lastActivity,
		OpenIssuesCount: 3,
		WebURL:          "https://gitlab.example.org/mpd-a1-team1",
	}

	assignmentCfg := &config.AssignmentConfig{
		Release: &config.Release{
			MergeRequest: &config.ReleaseMergeRequest{
				SourceBranch: "develop",
				TargetBranch: "main",
				HasPipeline:  true,
			},
			DockerImages: []string{"registry/app", "registry/missing"},
		},
	}

	projectName, report := client.projectReport(assignmentCfg, project)

	if projectName != "mpd-a1-team1" {
		t.Fatalf("projectName = %q", projectName)
	}
	if report == nil {
		t.Fatal("report is nil")
	}
	if report.Commits != 2 {
		t.Fatalf("commits = %d, want 2", report.Commits)
	}
	if report.LastCommit == nil || report.LastCommit.Title != "latest" {
		t.Fatalf("last commit = %#v", report.LastCommit)
	}
	if report.OpenMergeRequestsCount != 1 {
		t.Fatalf("open merge requests = %d", report.OpenMergeRequestsCount)
	}
	if !report.IsActive {
		t.Fatal("project should be marked active")
	}
	if len(report.Members) != 1 {
		t.Fatalf("members len = %d, want 1", len(report.Members))
	}

	if report.Release == nil || report.Release.MergeRequest == nil {
		t.Fatalf("release merge request = %#v", report.Release)
	}
	if !report.Release.MergeRequest.Found {
		t.Fatal("release merge request should be found")
	}
	if report.Release.MergeRequest.PipelineStatus != "success" {
		t.Fatalf("pipeline status = %q, want success", report.Release.MergeRequest.PipelineStatus)
	}

	if report.Release.DockerImages == nil {
		t.Fatal("docker images report should be set")
	}
	if report.Release.DockerImages.Status != "1 of 2 available" {
		t.Fatalf("docker images status = %q", report.Release.DockerImages.Status)
	}
	if len(report.Release.DockerImages.Images) != 1 {
		t.Fatalf("docker images len = %d, want 1", len(report.Release.DockerImages.Images))
	}
	if report.Release.DockerImages.Images[0].Wanted != "registry/app" {
		t.Fatalf("docker image wanted = %q", report.Release.DockerImages.Images[0].Wanted)
	}
}
