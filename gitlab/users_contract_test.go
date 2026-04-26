package gitlab

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/obcode/glabs/v2/config"
)

// ---- getUser ----------------------------------------------------------------

func TestGetUser_ByID_Found(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/users/42" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 42, "name": "Alice", "username": "alice",
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	id := 42
	user, err := client.getUser(&config.Student{Id: &id})
	if err != nil {
		t.Fatalf("getUser(byID) error = %v", err)
	}
	if user == nil || user.ID != 42 {
		t.Fatalf("user = %#v, want id 42", user)
	}
}

func TestGetUser_ByUsername_Found(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/users" {
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "name": "Alice", "username": "alice"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	username := "alice"
	user, err := client.getUser(&config.Student{Username: &username})
	if err != nil {
		t.Fatalf("getUser(byUsername) error = %v", err)
	}
	if user == nil || user.ID != 1 {
		t.Fatalf("user = %#v, want id 1", user)
	}
}

func TestGetUser_ByEmail_Found(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/users" {
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 2, "name": "Bob", "username": "bob", "email": "bob@example.com"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	email := "bob@example.com"
	user, err := client.getUser(&config.Student{Email: &email})
	if err != nil {
		t.Fatalf("getUser(byEmail) error = %v", err)
	}
	if user == nil || user.ID != 2 {
		t.Fatalf("user = %#v, want id 2", user)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/users" {
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	username := "ghost"
	user, err := client.getUser(&config.Student{Username: &username})
	if err == nil {
		t.Fatal("getUser() expected error for not found, got nil")
	}
	if user != nil {
		t.Fatalf("user = %#v, want nil", user)
	}
}

func TestGetUser_MultipleFound(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/users" {
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 1, "username": "alice"},
				{"id": 2, "username": "alice2"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	username := "alice"
	user, err := client.getUser(&config.Student{Username: &username})
	if err == nil {
		t.Fatal("getUser() expected error for multiple users, got nil")
	}
	if user != nil {
		t.Fatalf("user = %#v, want nil", user)
	}
}

// ---- getUserID --------------------------------------------------------------

func TestGetUserID_Success(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/users" {
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{
				{"id": 5, "username": "alice"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	username := "alice"
	id, err := client.getUserID(&config.Student{Username: &username})
	if err != nil {
		t.Fatalf("getUserID() error = %v", err)
	}
	if id != 5 {
		t.Fatalf("getUserID() = %d, want 5", id)
	}
}

func TestGetUserID_Error(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/users" {
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	username := "ghost"
	_, err := client.getUserID(&config.Student{Username: &username})
	if err == nil {
		t.Fatal("getUserID() expected error, got nil")
	}
}

// ---- addMember --------------------------------------------------------------

func TestAddMember_NewMember_Success(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/10/members/all/5":
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "404 Not Found"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/10/members":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 5, "username": "alice", "access_level": 30,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{AccessLevel: config.Developer}
	info, err := client.addMember(cfg, 10, 5)
	if err != nil {
		t.Fatalf("addMember() error = %v", err)
	}
	if info == "" {
		t.Fatal("addMember() returned empty info")
	}
}

func TestAddMember_AlreadyOwner(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/10/members/all/5" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 5, "username": "alice", "access_level": 50,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{AccessLevel: config.Developer}
	info, err := client.addMember(cfg, 10, 5)
	if err != nil {
		t.Fatalf("addMember() error = %v", err)
	}
	if info != "already owner" {
		t.Fatalf("addMember() = %q, want \"already owner\"", info)
	}
}

func TestAddMember_AlreadyMember_SameLevel(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/10/members/all/5" {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 5, "username": "alice", "access_level": 30,
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{AccessLevel: config.Developer}
	info, err := client.addMember(cfg, 10, 5)
	if err != nil {
		t.Fatalf("addMember() error = %v", err)
	}
	if info == "" {
		t.Fatal("addMember() returned empty info for already-member case")
	}
}

func TestAddMember_AlreadyMember_DifferentLevel_Updated(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/10/members/all/5":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 5, "username": "alice", "access_level": 20, // reporter
			})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v4/projects/10/members/5":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"id": 5, "username": "alice", "access_level": 30,
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{AccessLevel: config.Developer}
	info, err := client.addMember(cfg, 10, 5)
	if err != nil {
		t.Fatalf("addMember() error = %v", err)
	}
	if info == "" {
		t.Fatal("addMember() returned empty info for level-change case")
	}
}

func TestAddMember_AddFails(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/10/members/all/5":
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "404 Not Found"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/10/members":
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "403 Forbidden"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{AccessLevel: config.Developer}
	_, err := client.addMember(cfg, 10, 5)
	if err == nil {
		t.Fatal("addMember() expected error on forbidden, got nil")
	}
}
