package gitlab

import (
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/config"
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

	if err := client.replicateIssue(source, target, 7); err != nil {
		t.Fatalf("replicateIssue() error = %v", err)
	}
}

func TestReplicateIssue_GetIssueFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	source := &gitlabapi.Project{ID: 1}
	target := &gitlabapi.Project{ID: 2}

	err := client.replicateIssue(source, target, 7)
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

	source := &gitlabapi.Project{ID: 1}
	target := &gitlabapi.Project{ID: 2}

	err := client.replicateIssue(source, target, 7)
	if err == nil {
		t.Fatal("expected error when creating issue fails")
	}
}
