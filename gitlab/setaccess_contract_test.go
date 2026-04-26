package gitlab

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/v2/config"
)

// ---- Setaccess (top-level) --------------------------------------------------

func TestSetaccess_GroupNotFound_Exits(t *testing.T) {
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
	assertExitCode(t, 1, func() { client.Setaccess(cfg) })
}

func TestSetaccess_InvalidPer_Exits(t *testing.T) {
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
	assertExitCode(t, 1, func() { client.Setaccess(cfg) })
}

// ---- setaccessPerStudent ----------------------------------------------------

func TestSetaccessPerStudent_NoStudents(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:      "mpd",
		Name:        "blatt01",
		Path:        "mpd/ss26/blatt-01",
		Per:         config.PerStudent,
		Students:    []*config.Student{},
		AccessLevel: config.Developer,
	}
	client.setaccessPerStudent(cfg)
}

func TestSetaccessPerStudent_GetProjectFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	username := "alice"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		Per:                   config.PerStudent,
		UseCoursenameAsPrefix: true,
		Students:              []*config.Student{{Username: &username, Raw: "alice"}},
		AccessLevel:           config.Developer,
	}
	client.setaccessPerStudent(cfg)
}

func TestSetaccessPerStudent_Success_NewMember(t *testing.T) {
	pj := `{"id":10,"name":"mpd-blatt01-alice","path_with_namespace":"mpd/ss26/blatt-01/mpd-blatt01-alice","ssh_url_to_repo":"git@example.com:p.git"}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		// GetProject
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "mpd-blatt01-alice"):
			_, _ = w.Write([]byte(pj))
		// ListUsers
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/users":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 5, "username": "alice"},
			})
		// GetInheritedProjectMember
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/10/members/all/5":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		// AddProjectMember
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/10/members":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 5, "username": "alice", "access_level": 30,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	username := "alice"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		Per:                   config.PerStudent,
		UseCoursenameAsPrefix: true,
		Students:              []*config.Student{{Username: &username, Raw: "alice"}},
		AccessLevel:           config.Developer,
	}
	client.setaccessPerStudent(cfg)
}

// ---- setaccessPerGroup ------------------------------------------------------

func TestSetaccessPerGroup_NoGroups(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:      "mpd",
		Name:        "blatt01",
		Path:        "mpd/ss26/blatt-01",
		Per:         config.PerGroup,
		Groups:      []*config.Group{},
		AccessLevel: config.Developer,
	}
	client.setaccessPerGroup(cfg)
}

func TestSetaccessPerGroup_Success(t *testing.T) {
	pj := `{"id":11,"name":"mpd-blatt01-team1","path_with_namespace":"mpd/ss26/blatt-01/mpd-blatt01-team1","ssh_url_to_repo":"git@example.com:p.git"}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "mpd-blatt01-team1"):
			_, _ = w.Write([]byte(pj))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/users":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 6, "username": "alice"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/11/members/all/6":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/11/members":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 6, "username": "alice", "access_level": 30,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	alice := "alice"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		Per:                   config.PerGroup,
		UseCoursenameAsPrefix: true,
		Groups: []*config.Group{
			{Name: "team1", Members: []*config.Student{{Username: &alice, Raw: "alice"}}},
		},
		AccessLevel: config.Developer,
	}
	client.setaccessPerGroup(cfg)
}

// ---- inviteByEmail ----------------------------------------------------------

func TestInviteByEmail_Success(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/10/invitations" {
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{AccessLevel: config.Developer}
	info, err := client.inviteByEmail(cfg, 10, "newuser@example.com")
	if err != nil {
		t.Fatalf("inviteByEmail() error = %v", err)
	}
	if info == "" {
		t.Fatal("inviteByEmail() returned empty info")
	}
}

func TestInviteByEmail_APIError(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/10/invitations" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{AccessLevel: config.Developer}
	_, err := client.inviteByEmail(cfg, 10, "user@example.com")
	if err == nil {
		t.Fatal("inviteByEmail() expected error on 403, got nil")
	}
}

func TestInviteByEmail_StatusNotSuccess(t *testing.T) {
	email := "user@example.com"
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/10/invitations" {
			resp := fmt.Sprintf(`{"status":"error","message":{%q:"User already exists"}}`, email)
			_, _ = w.Write([]byte(resp))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{AccessLevel: config.Developer}
	_, err := client.inviteByEmail(cfg, 10, email)
	if err == nil {
		t.Fatal("inviteByEmail() expected error when status != success, got nil")
	}
}

// ---- setaccess (inviteByEmail path) -----------------------------------------

func TestSetaccessPerStudent_UserNotFound_InviteByEmail(t *testing.T) {
	// When getUserID fails but student has email → try invite
	pj := `{"id":12,"name":"mpd-blatt01-bob","path_with_namespace":"mpd/ss26/blatt-01/mpd-blatt01-bob","ssh_url_to_repo":"git@example.com:p.git"}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "mpd-blatt01-bob"):
			_, _ = w.Write([]byte(pj))
		// ListUsers returns empty (user not found)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/users":
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		// Invite by email succeeds
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/12/invitations":
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	email := "bob@example.com"
	cfg := &config.AssignmentConfig{
		Course:                "mpd",
		Name:                  "blatt01",
		Path:                  "mpd/ss26/blatt-01",
		Per:                   config.PerStudent,
		UseCoursenameAsPrefix: true,
		Students:              []*config.Student{{Email: &email, Raw: "bob@example.com"}},
		AccessLevel:           config.Developer,
	}
	client.setaccessPerStudent(cfg)
}
