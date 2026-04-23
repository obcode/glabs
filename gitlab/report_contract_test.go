package gitlab

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/obcode/glabs/config"
)

// makeFullReportHandler returns a handler that mocks all calls needed by report().
// It serves: getGroupID, ListGroupProjects, and full projectReport for one project.
func makeFullReportHandler(groupID, projectID int64, projectName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		// getGroupIDByFullPath
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups":
			json.NewEncoder(w).Encode([]map[string]interface{}{ //nolint
				{"id": groupID, "full_path": "mpd/ss26/blatt-01"},
			})

		// ListGroupProjects - no next page
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/projects") &&
			strings.Contains(r.URL.Path, "/groups/"):
			json.NewEncoder(w).Encode([]map[string]interface{}{ //nolint
				{"id": projectID, "name": projectName,
					"created_at":          "2026-04-01T09:00:00Z",
					"last_activity_at":    "2026-04-02T10:00:00Z",
					"path_with_namespace": "mpd/ss26/blatt-01/" + projectName,
					"ssh_url_to_repo":     "git@example.com:mpd/ss26/blatt-01/" + projectName + ".git"},
			})

		// ListBranches
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/repository/branches"):
			_, _ = w.Write([]byte(`[{"name":"main"}]`))

		// ListCommits
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/repository/commits"):
			_, _ = w.Write([]byte(`[{"title":"first commit","committer_name":"alice","committed_date":"2026-04-01T10:00:00Z","web_url":"https://gitlab.example.org/c1"}]`))

		// ListProjectMembers
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/members"):
			_, _ = w.Write([]byte(`[{"id":1,"name":"Alice","username":"alice","web_url":"https://gitlab.example.org/alice"}]`))

		// ListProjectMergeRequests
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/merge_requests"):
			_, _ = w.Write([]byte(`[]`))

		// Registry (docker images) - return empty
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/registry/repositories"):
			_, _ = w.Write([]byte(`[]`))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// ---- report (internal) ------------------------------------------------------

func TestReport_GroupNotFound_ReturnsNil(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	result := client.report(cfg)
	if result != nil {
		t.Fatalf("report() = %#v, want nil", result)
	}
}

func TestReport_ListGroupProjectsFails_ReturnsNil(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups":
			_, _ = w.Write([]byte(`[{"id":1,"full_path":"mpd/ss26/blatt-01"}]`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"500 Internal Server Error"}`))
		}
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	result := client.report(cfg)
	if result != nil {
		t.Fatalf("report() = %#v, want nil after ListGroupProjects failure", result)
	}
}

func TestReport_HappyPath_NoProjects(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups":
			_, _ = w.Write([]byte(`[{"id":1,"full_path":"mpd/ss26/blatt-01"}]`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/groups/1/projects"):
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	result := client.report(cfg)
	if result == nil {
		t.Fatal("report() returned nil, want non-nil")
	}
	if result.Course != "mpd" {
		t.Fatalf("Course = %q, want mpd", result.Course)
	}
	if len(result.Projects) != 0 {
		t.Fatalf("Projects = %d, want 0", len(result.Projects))
	}
}

func TestReport_HappyPath_WithProject(t *testing.T) {
	client := newContractClient(t, makeFullReportHandler(1, 10, "mpd-blatt01-alice"))

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	result := client.report(cfg)
	if result == nil {
		t.Fatal("report() returned nil, want non-nil")
	}
	if len(result.Projects) != 1 {
		t.Fatalf("Projects = %d, want 1", len(result.Projects))
	}
}

func TestReport_HappyPath_WithRelease(t *testing.T) {
	client := newContractClient(t, makeFullReportHandler(1, 10, "mpd-blatt01-alice"))

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
		Release: &config.Release{
			MergeRequest: &config.MergeRequest{SourceBranch: "develop", TargetBranch: "main"},
			DockerImages: []string{"myimage:latest"},
		},
	}
	result := client.report(cfg)
	if result == nil {
		t.Fatal("report() returned nil, want non-nil")
	}
	if !result.HasReleaseMergeRequest {
		t.Fatal("HasReleaseMergeRequest should be true")
	}
	if !result.HasReleaseDockerImages {
		t.Fatal("HasReleaseDockerImages should be true")
	}
}

// ---- Report (text output) ---------------------------------------------------

func TestReport_TextTemplate_ToStdout(t *testing.T) {
	client := newContractClient(t, makeFullReportHandler(1, 11, "mpd-blatt01-alice"))

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	// nil template → uses default text template; nil output → uses stdout
	client.Report(cfg, nil, nil)
}

func TestReport_TextTemplate_ToFile(t *testing.T) {
	client := newContractClient(t, makeFullReportHandler(1, 12, "mpd-blatt01-alice"))

	outFile := filepath.Join(t.TempDir(), "report.txt")
	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	client.Report(cfg, nil, &outFile)

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Report() wrote empty file")
	}
}

func TestReport_CustomTemplate_ToFile(t *testing.T) {
	// Write a custom template file
	tmplContent := `Course: {{.Course}}, Assignment: {{.Assignment}}`
	tmplFile := filepath.Join(t.TempDir(), "tmpl.txt")
	if err := os.WriteFile(tmplFile, []byte(tmplContent), 0644); err != nil {
		t.Fatalf("writing template: %v", err)
	}

	client := newContractClient(t, makeFullReportHandler(1, 13, "mpd-blatt01-alice"))

	outFile := filepath.Join(t.TempDir(), "out.txt")
	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	client.Report(cfg, &tmplFile, &outFile)

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if !strings.Contains(string(data), "mpd") {
		t.Fatalf("Report() output missing course name: %q", data)
	}
}

// ---- ReportHTML -------------------------------------------------------------

func TestReportHTML_DefaultTemplate_ToStdout(t *testing.T) {
	client := newContractClient(t, makeFullReportHandler(1, 14, "mpd-blatt01-alice"))

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	client.ReportHTML(cfg, nil, nil)
}

func TestReportHTML_DefaultTemplate_ToFile(t *testing.T) {
	client := newContractClient(t, makeFullReportHandler(1, 15, "mpd-blatt01-alice"))

	outFile := filepath.Join(t.TempDir(), "report.html")
	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	client.ReportHTML(cfg, nil, &outFile)

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("ReportHTML() wrote empty file")
	}
}

// ---- ReportJSON -------------------------------------------------------------

func TestReportJSON_ToStdout(t *testing.T) {
	client := newContractClient(t, makeFullReportHandler(1, 16, "mpd-blatt01-alice"))

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	client.ReportJSON(cfg, nil)
}

func TestReportJSON_ToFile(t *testing.T) {
	client := newContractClient(t, makeFullReportHandler(1, 17, "mpd-blatt01-alice"))

	outFile := filepath.Join(t.TempDir(), "report.json")
	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
	}
	client.ReportJSON(cfg, &outFile)

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("ReportJSON() wrote invalid JSON: %v", err)
	}
	if parsed["course"] != "mpd" {
		t.Fatalf("JSON course = %q, want mpd", parsed["course"])
	}
}
