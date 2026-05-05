package gitlab

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/v2/config"
	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

func TestGetStartercodeProject_ParseSSHURL(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v4/projects/") {
			_, _ = w.Write([]byte(`{"id":42,"path_with_namespace":"mpd/startercode/blatt-01"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{URL: "git@gitlab.example.org:mpd/startercode/blatt-01.git"},
	}

	project, err := client.getStartercodeProject(cfg)
	if err != nil {
		t.Fatalf("getStartercodeProject() error = %v", err)
	}
	if project == nil || project.ID != 42 {
		t.Fatalf("unexpected project: %#v", project)
	}
}

func TestGetStartercodeProject_ParseHTTPSURLWithoutGitSuffix(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v4/projects/") {
			_, _ = w.Write([]byte(`{"id":43,"path_with_namespace":"mpd/startercode/blatt-02"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{URL: "https://gitlab.example.org/mpd/startercode/blatt-02"},
	}

	project, err := client.getStartercodeProject(cfg)
	if err != nil {
		t.Fatalf("getStartercodeProject() error = %v", err)
	}
	if project == nil || project.ID != 43 {
		t.Fatalf("unexpected project: %#v", project)
	}
}

func TestGetStartercodeProject_InvalidURL(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{URL: "not-a-valid-url"},
	}

	_, err := client.getStartercodeProject(cfg)
	if err == nil {
		t.Fatal("expected parse error for invalid startercode URL")
	}
}

func TestGetStartercodeProject_ProjectLookupFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{URL: "https://gitlab.example.org/mpd/startercode/blatt-03.git"},
	}

	_, err := client.getStartercodeProject(cfg)
	if err == nil {
		t.Fatal("expected project lookup error")
	}
}

func TestReplicateIssue_Success(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/issues/7":
			_, _ = w.Write([]byte(`{"id":7001,"iid":7,"title":"Fix tests","description":"Please fix tests"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/2/issues":
			_, _ = w.Write([]byte(`{"id":9901,"iid":99}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-01"}
	target := &gitlabapi.Project{ID: 2, PathWithNamespace: "mpd/ss26/blatt-01/team1"}

	if _, err := client.replicateIssue(source, target, 7, false); err != nil {
		t.Fatalf("replicateIssue() error = %v", err)
	}
}

func TestReplicateIssue_GetIssueFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/issues/7" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-01"}
	target := &gitlabapi.Project{ID: 2}

	_, err := client.replicateIssue(source, target, 7, false)
	if err == nil {
		t.Fatal("expected error when loading issue fails")
	}
}

func TestReplicateIssue_CreateIssueFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/issues/7":
			_, _ = w.Write([]byte(`{"id":7001,"iid":7,"title":"Fix tests","description":"Please fix tests"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/2/issues":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-01"}
	target := &gitlabapi.Project{ID: 2}

	_, err := client.replicateIssue(source, target, 7, false)
	if err == nil {
		t.Fatal("expected error when creating issue fails")
	}
}

func TestResolveIssueNumbersForReplication_WithChildTasks(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v4/projects/1/issues/") {
			if strings.HasSuffix(r.URL.Path, "/2") {
				_, _ = w.Write([]byte(`{"id":2002,"iid":2,"title":"Aufgabenstellung","description":"Root"}`))
				return
			}
			if strings.HasSuffix(r.URL.Path, "/5") {
				_, _ = w.Write([]byte(`{"id":2005,"iid":5,"title":"Teilaufgabe 1","description":"Child 1"}`))
				return
			}
			if strings.HasSuffix(r.URL.Path, "/6") {
				_, _ = w.Write([]byte(`{"id":2006,"iid":6,"title":"Teilaufgabe 2","description":"Child 2"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if r.Method != http.MethodPost || r.URL.Path != "/api/graphql" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		iid, _ := req.Variables["iid"].(string)
		switch iid {
		case "2":
			_, _ = w.Write([]byte(`{"data":{"project":{"issue":{"workItem":{"widgets":[{"children":{"nodes":[{"iid":"5"},{"iid":"6"}]}}]}}}}}`))
			return
		case "5", "6":
			_, _ = w.Write([]byte(`{"data":{"project":{"issue":{"workItem":{"widgets":[]}}}}}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	source := &gitlabapi.Project{ID: 1, PathWithNamespace: "mpd/startercode/blatt-07"}

	numbers, err := client.resolveIssueNumbersForReplication(source, []int{2}, true)
	if err != nil {
		t.Fatalf("resolveIssueNumbersForReplication() error = %v", err)
	}

	if len(numbers) != 3 || numbers[0] != 2 || numbers[1] != 5 || numbers[2] != 6 {
		t.Fatalf("resolved issue numbers = %#v, want [2 5 6]", numbers)
	}
}
