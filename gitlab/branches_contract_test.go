package gitlab

import (
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/config"
	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

func TestSyncConfiguredBranches_ReturnsErrorWhenProtectionFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v4/projects/1":
			_, _ = w.Write([]byte(`{"id":1,"name":"repo"}`))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/api/v4/projects/1/protected_branches/main"):
			_, _ = w.Write([]byte(`{"id":1,"name":"main","push_access_levels":[{"id":10,"access_level":40}],"merge_access_levels":[{"id":11,"access_level":40}],"unprotect_access_levels":[{"id":12,"access_level":40}]}`))
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/api/v4/projects/1/protected_branches/main"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/api/v4/projects/1/protected_branches"):
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"500 Internal Server Error"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Branches: []config.BranchRule{{Name: "main", MergeOnly: true, Default: true}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "repo"}

	err := client.syncConfiguredBranches(cfg, project, "main")
	if err == nil {
		t.Fatal("syncConfiguredBranches() expected error when branch protection fails, got nil")
	}
}
