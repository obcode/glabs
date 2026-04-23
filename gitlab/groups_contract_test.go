package gitlab

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/config"
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
