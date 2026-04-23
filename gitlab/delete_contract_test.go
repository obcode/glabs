package gitlab

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/config"
)

// groupSearchHandler returns a handler that mocks getGroupID + Search.ProjectsByGroup + DeleteProject.
// groupID: the group to return for path "mpd/ss26/blatt-01"
// projectID: the project to return from group search, 0 → empty results
// deletePath: the DELETE path that should succeed
func makeDeleteHandler(groupID, projectID int64, projectName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		// getGroupIDByFullPath → SearchGroup
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups":
			fmt.Fprintf(w, `[{"id":%d,"full_path":"mpd/ss26/blatt-01"}]`, groupID)

		// Search.ProjectsByGroup
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, fmt.Sprintf("/api/v4/groups/%d/search", groupID)):
			if projectID == 0 {
				_, _ = w.Write([]byte(`[]`))
			} else {
				fmt.Fprintf(w, `[{"id":%d,"name":%q}]`, projectID, projectName)
			}

		// DeleteProject
		case r.Method == http.MethodDelete && r.URL.Path == fmt.Sprintf("/api/v4/projects/%d", projectID):
			w.WriteHeader(http.StatusAccepted)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// ---- Delete (top-level) -----------------------------------------------------

func TestDelete_GroupNotFound_Exits(t *testing.T) {
	defer withExitCapture(t)()

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Path:   "mpd/ss26/blatt-01",
		Per:    config.PerStudent,
	}
	assertExitCode(t, 1, func() { client.Delete(cfg) })
}

func TestDelete_InvalidPer_Exits(t *testing.T) {
	defer withExitCapture(t)()

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups" {
			_, _ = w.Write([]byte(`[{"id":1,"full_path":"mpd/ss26/blatt-01"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Path:   "mpd/ss26/blatt-01",
		Per:    config.PerFailed,
	}
	assertExitCode(t, 1, func() { client.Delete(cfg) })
}

// ---- deletePerStudent -------------------------------------------------------

func TestDeletePerStudent_NoStudents(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:   "mpd",
		Name:     "blatt01",
		Path:     "mpd/ss26/blatt-01",
		Per:      config.PerStudent,
		Students: []*config.Student{},
	}
	client.deletePerStudent(cfg, 1)
}

func TestDeletePerStudent_ProjectFound_Deleted(t *testing.T) {
	username := "alice"
	client := newContractClient(t, makeDeleteHandler(1, 42, "mpd-blatt01-alice"))

	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		Per:                   config.PerStudent,
		UseCoursenameAsPrefix: true,
		Students:              []*config.Student{{Username: &username, Raw: "alice"}},
	}
	client.deletePerStudent(cfg, 1)
}

func TestDeletePerStudent_ProjectNotFound(t *testing.T) {
	username := "alice"
	client := newContractClient(t, makeDeleteHandler(1, 0, ""))

	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		Per:                   config.PerStudent,
		UseCoursenameAsPrefix: true,
		Students:              []*config.Student{{Username: &username, Raw: "alice"}},
	}
	// Project not found → delete is a no-op
	client.deletePerStudent(cfg, 1)
}

// ---- deletePerGroup ---------------------------------------------------------

func TestDeletePerGroup_NoGroups(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		Per:    config.PerGroup,
		Groups: []*config.Group{},
	}
	client.deletePerGroup(cfg, 1)
}

func TestDeletePerGroup_ProjectFound_Deleted(t *testing.T) {
	alice := "alice"
	client := newContractClient(t, makeDeleteHandler(1, 43, "mpd-blatt01-team1"))

	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		Per:                   config.PerGroup,
		UseCoursenameAsPrefix: true,
		Groups: []*config.Group{
			{Name: "team1", Members: []*config.Student{{Username: &alice, Raw: "alice"}}},
		},
	}
	client.deletePerGroup(cfg, 1)
}

// ---- delete (low-level) -----------------------------------------------------

func TestDelete_LowLevel_SearchError(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"500 Internal Server Error"}`))
	})

	// Should log error and return without panicking
	client.delete(1, "myrepo")
}

func TestDelete_LowLevel_EmptyResults(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/search") {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// No results → nothing to delete
	client.delete(1, "myrepo")
}

func TestDelete_LowLevel_NameMismatch(t *testing.T) {
	// Search returns a project but with a different name → no deletion
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/search") {
			_, _ = w.Write([]byte(`[{"id":99,"name":"different-name"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	client.delete(1, "myrepo")
}

func TestDelete_LowLevel_DeleteFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/search"):
			_, _ = w.Write([]byte(`[{"id":99,"name":"myrepo"}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v4/projects/99":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	// delete fails → logs error, no panic
	client.delete(1, "myrepo")
}
