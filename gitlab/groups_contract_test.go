package gitlab

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/v3/config"
)

func TestGetGroupIDByFullPath_FindsGroup(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups" {
			_, _ = w.Write([]byte(`[
				{"id":1,"full_path":"other/path"},
				{"id":42,"full_path":"mpd/ss26/blatt-01"}
			]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	id, err := client.getGroupIDByFullPath("mpd/ss26/blatt-01")
	if err != nil {
		t.Fatalf("getGroupIDByFullPath() returned error: %v", err)
	}
	if id != 42 {
		t.Fatalf("group id = %d, want 42", id)
	}
}

func TestGetGroupIDByFullPath_ReturnsErrorWhenNotFound(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups" {
			_, _ = w.Write([]byte(`[{"id":1,"full_path":"other/path"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := client.getGroupIDByFullPath("mpd/ss26/blatt-01")
	if err == nil {
		t.Fatal("getGroupIDByFullPath() expected error, got nil")
	}
}

func TestCreateGroup_WithParentGroup(t *testing.T) {
	var createBody string

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups":
			_, _ = w.Write([]byte(`[{"id":41,"full_path":"mpd/ss26"}]`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/groups":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			createBody = string(body)
			_, _ = w.Write([]byte(`{"id":99,"full_path":"mpd/ss26/blatt-01"}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	assignmentCfg := &config.AssignmentConfig{Course: "mpd", Path: "mpd/ss26/blatt-01"}
	id, err := client.createGroup(assignmentCfg)
	if err != nil {
		t.Fatalf("createGroup() returned error: %v", err)
	}
	if id != 99 {
		t.Fatalf("group id = %d, want 99", id)
	}

	if !strings.Contains(createBody, `"name":"blatt-01"`) && !strings.Contains(createBody, "name=blatt-01") {
		t.Fatalf("create group request body missing name: %q", createBody)
	}
	if !strings.Contains(createBody, `"path":"blatt-01"`) && !strings.Contains(createBody, "path=blatt-01") {
		t.Fatalf("create group request body missing path: %q", createBody)
	}
	if !strings.Contains(createBody, `"parent_id":41`) && !strings.Contains(createBody, "parent_id=41") {
		t.Fatalf("create group request body missing parent_id: %q", createBody)
	}
}

// TestCreateGroup_InheritsPrivateVisibilityFromParent guards against the 400
// GitLab answers with when a subgroup would be less restrictive than its
// parent: "internal is not allowed since the parent group has a private
// visibility".
func TestCreateGroup_InheritsPrivateVisibilityFromParent(t *testing.T) {
	var createBody string

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups":
			_, _ = w.Write([]byte(`[{"id":41,"full_path":"mpd/ss26","visibility":"private"}]`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/groups":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			createBody = string(body)
			_, _ = w.Write([]byte(`{"id":99,"full_path":"mpd/ss26/blatt-01"}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	assignmentCfg := &config.AssignmentConfig{Course: "mpd", Path: "mpd/ss26/blatt-01"}
	if _, err := client.createGroup(assignmentCfg); err != nil {
		t.Fatalf("createGroup() returned error: %v", err)
	}

	if !strings.Contains(createBody, `"visibility":"private"`) && !strings.Contains(createBody, "visibility=private") {
		t.Fatalf("create group request body should request private visibility: %q", createBody)
	}
}

func TestCreateGroup_UsesInternalVisibilityUnderInternalParent(t *testing.T) {
	var createBody string

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups":
			_, _ = w.Write([]byte(`[{"id":41,"full_path":"mpd/ss26","visibility":"internal"}]`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/groups":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			createBody = string(body)
			_, _ = w.Write([]byte(`{"id":99,"full_path":"mpd/ss26/blatt-01"}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	assignmentCfg := &config.AssignmentConfig{Course: "mpd", Path: "mpd/ss26/blatt-01"}
	if _, err := client.createGroup(assignmentCfg); err != nil {
		t.Fatalf("createGroup() returned error: %v", err)
	}

	if !strings.Contains(createBody, `"visibility":"internal"`) && !strings.Contains(createBody, "visibility=internal") {
		t.Fatalf("create group request body should request internal visibility: %q", createBody)
	}
}

// TestCreateGroup_OmitsVisibilityWhenParentVisibilityUnknown covers the case
// where the API does not report the parent's visibility: sending no visibility
// at all lets GitLab pick one that is valid for the parent, which can never be
// rejected.
func TestCreateGroup_OmitsVisibilityWhenParentVisibilityUnknown(t *testing.T) {
	var createBody string

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups":
			_, _ = w.Write([]byte(`[{"id":41,"full_path":"mpd/ss26"}]`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/groups":
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("ReadAll failed: %v", err)
			}
			createBody = string(body)
			_, _ = w.Write([]byte(`{"id":99,"full_path":"mpd/ss26/blatt-01"}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	assignmentCfg := &config.AssignmentConfig{Course: "mpd", Path: "mpd/ss26/blatt-01"}
	if _, err := client.createGroup(assignmentCfg); err != nil {
		t.Fatalf("createGroup() returned error: %v", err)
	}

	if strings.Contains(createBody, "visibility") {
		t.Fatalf("create group request body should not contain a visibility: %q", createBody)
	}
}

// TestCreateGroup_ReturnsErrorWhenCreationFails is the regression test for the
// panic in issue #147: the failed creation was only logged, and the nil group
// it returned was dereferenced by the caller.
func TestCreateGroup_ReturnsErrorWhenCreationFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups":
			_, _ = w.Write([]byte(`[{"id":41,"full_path":"mpd/ss26","visibility":"private"}]`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/groups":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"message":{"visibility_level":["internal is not allowed since the parent group has a private visibility."]}}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	assignmentCfg := &config.AssignmentConfig{Course: "mpd", Path: "mpd/ss26/blatt-01"}
	id, err := client.createGroup(assignmentCfg)
	if err == nil {
		t.Fatal("createGroup() expected error, got nil")
	}
	if id != 0 {
		t.Fatalf("group id = %d, want 0 on error", id)
	}
	if !strings.Contains(err.Error(), "mpd/ss26/blatt-01") {
		t.Fatalf("error should name the group path, got: %v", err)
	}
}

func TestCreateGroup_ReturnsErrorWhenParentDoesNotExist(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups" {
			_, _ = w.Write([]byte(`[]`))
			return
		}
		if r.Method == http.MethodPost && r.URL.Path == "/api/v4/groups" {
			t.Fatal("createGroup() must not create a group when the parent cannot be resolved")
		}
		w.WriteHeader(http.StatusNotFound)
	})

	assignmentCfg := &config.AssignmentConfig{Course: "mpd", Path: "mpd/ss26/blatt-01"}
	if _, err := client.createGroup(assignmentCfg); err == nil {
		t.Fatal("createGroup() expected error, got nil")
	}
}
