package gitlab

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/config"
)

func TestGetProjectByName_FindsSingleProject(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects" {
			_, _ = w.Write([]byte(`[{"id":1,"name":"repo","path_with_namespace":"mpd/ss26/repo"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	project, err := client.getProjectByName("mpd/ss26/repo")
	if err != nil {
		t.Fatalf("getProjectByName() returned error: %v", err)
	}
	if project == nil || project.ID != 1 {
		t.Fatalf("project = %#v", project)
	}
}

func TestGetProjectByName_SelectsExactMatchOnMultipleResults(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects" {
			_, _ = w.Write([]byte(`[
				{"id":1,"name":"repo","path_with_namespace":"other/path/repo"},
				{"id":2,"name":"repo","path_with_namespace":"mpd/ss26/repo"}
			]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	project, err := client.getProjectByName("mpd/ss26/repo")
	if err != nil {
		t.Fatalf("getProjectByName() returned error: %v", err)
	}
	if project == nil || project.ID != 2 {
		t.Fatalf("project = %#v", project)
	}
}

func TestGetProjectByName_ReturnsErrorWhenNotFound(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	project, err := client.getProjectByName("mpd/ss26/repo")
	if err == nil {
		t.Fatal("getProjectByName() expected error, got nil")
	}
	if project != nil {
		t.Fatalf("project = %#v, want nil", project)
	}
}

func TestGenerateProject_CreatesProject(t *testing.T) {
	var createBody string

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			createBody = string(body)
			_, _ = w.Write([]byte(`{"id":11,"name":"repo-a","path_with_namespace":"mpd/ss26/repo-a"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	assignmentCfg := &config.AssignmentConfig{
		Description:       "desc",
		Path:              "mpd/ss26",
		ContainerRegistry: true,
	}

	project, generated, err := client.generateProject(assignmentCfg, "repo-a", 123)
	if err != nil {
		t.Fatalf("generateProject() returned error: %v", err)
	}
	if !generated {
		t.Fatal("generateProject() generated = false, want true")
	}
	if project == nil || project.ID != 11 {
		t.Fatalf("project = %#v", project)
	}
	if !strings.Contains(createBody, `"name":"repo-a"`) && !strings.Contains(createBody, "name=repo-a") {
		t.Fatalf("create project request body missing name: %q", createBody)
	}
}

func TestGenerateProject_FallsBackToExistingProject(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":{"name":["has already been taken"]}}`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects":
			if got := r.URL.Query().Get("search"); !strings.Contains(got, "repo-a") {
				t.Fatalf("search query = %q", got)
			}
			_, _ = w.Write([]byte(`[{"id":21,"name":"repo-a","path_with_namespace":"mpd/ss26/repo-a"}]`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	assignmentCfg := &config.AssignmentConfig{Description: "desc", Path: "mpd/ss26"}

	project, generated, err := client.generateProject(assignmentCfg, "repo-a", 123)
	if err != nil {
		t.Fatalf("generateProject() returned error: %v", err)
	}
	if generated {
		t.Fatal("generateProject() generated = true, want false")
	}
	if project == nil || project.ID != 21 {
		t.Fatalf("project = %#v", project)
	}
}
