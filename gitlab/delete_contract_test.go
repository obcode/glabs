package gitlab

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/v3/config"
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
			_, _ = fmt.Fprintf(w, `[{"id":%d,"full_path":"mpd/ss26/blatt-01"}]`, groupID)

		// Search.ProjectsByGroup
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, fmt.Sprintf("/api/v4/groups/%d/search", groupID)):
			if projectID == 0 {
				_, _ = w.Write([]byte(`[]`))
			} else {
				_, _ = fmt.Fprintf(w, `[{"id":%d,"name":%q}]`, projectID, projectName)
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

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Path:   "mpd/ss26/blatt-01",
		Per:    config.PerStudent,
	}
	if err := client.Delete(cfg); err == nil {
		t.Fatal("expected an error")
	}
}

func TestDelete_InvalidPer_Exits(t *testing.T) {

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
	if err := client.Delete(cfg); err == nil {
		t.Fatal("expected an error")
	}
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

// ---- container registry tags --------------------------------------------------

// registryDeleteHandler mocks project search, a registry repository (id 7) with
// the given tags, and the delete endpoints. It records the DELETE requests in
// order; tagDeleteStatus is the status code returned for every tag delete.
func registryDeleteHandler(tags []string, tagDeleteStatus int, deletes *[]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/search"):
			_, _ = w.Write([]byte(`[{"id":99,"name":"myrepo"}]`))

		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/99/registry/repositories":
			_, _ = w.Write([]byte(`[{"id":7,"path":"mpd/myrepo/app"}]`))

		// Tags are served one per page so the pagination loop is exercised.
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/99/registry/repositories/7/tags":
			page := 1
			_, _ = fmt.Sscan(r.URL.Query().Get("page"), &page)
			if page < len(tags) {
				w.Header().Set("X-Next-Page", fmt.Sprint(page+1))
			}
			if page <= len(tags) {
				_, _ = fmt.Fprintf(w, `[{"name":%q}]`, tags[page-1])
			} else {
				_, _ = w.Write([]byte(`[]`))
			}

		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/v4/projects/99/registry/repositories/7/tags/"):
			*deletes = append(*deletes, "tag "+strings.TrimPrefix(r.URL.Path, "/api/v4/projects/99/registry/repositories/7/tags/"))
			w.WriteHeader(tagDeleteStatus)

		case r.Method == http.MethodDelete && r.URL.Path == "/api/v4/projects/99":
			*deletes = append(*deletes, "project")
			w.WriteHeader(http.StatusAccepted)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func TestDelete_LowLevel_RegistryTagsDeletedFirst(t *testing.T) {
	var deletes []string
	client := newContractClient(t, registryDeleteHandler([]string{"latest", "v1.0"}, http.StatusOK, &deletes))

	client.delete(1, "myrepo")

	want := []string{"tag latest", "tag v1.0", "project"}
	if strings.Join(deletes, ",") != strings.Join(want, ",") {
		t.Fatalf("deletes = %v, want %v", deletes, want)
	}
}

func TestDelete_LowLevel_RegistryTagDeleteFails_ProjectKept(t *testing.T) {
	var deletes []string
	client := newContractClient(t, registryDeleteHandler([]string{"latest", "v1.0"}, http.StatusForbidden, &deletes))

	client.delete(1, "myrepo")

	want := []string{"tag latest"}
	if strings.Join(deletes, ",") != strings.Join(want, ",") {
		t.Fatalf("deletes = %v, want %v", deletes, want)
	}
}

func TestDelete_LowLevel_RegistryUnavailable_ProjectStillDeleted(t *testing.T) {
	var deletes []string
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/search"):
			_, _ = w.Write([]byte(`[{"id":99,"name":"myrepo"}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v4/projects/99":
			deletes = append(deletes, "project")
			w.WriteHeader(http.StatusAccepted)
		default:
			// Registry listing → 404, as for a project with the registry disabled.
			w.WriteHeader(http.StatusNotFound)
		}
	})

	client.delete(1, "myrepo")

	if strings.Join(deletes, ",") != "project" {
		t.Fatalf("deletes = %v, want [project]", deletes)
	}
}
