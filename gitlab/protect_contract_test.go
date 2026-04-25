package gitlab

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/obcode/glabs/config"
	gitlabapi "gitlab.com/gitlab-org/api/client-go/v2"
)

// ---- ProtectToBranch (top-level) --------------------------------------------

func TestProtectToBranch_GroupNotFound_Exits(t *testing.T) {
	defer withExitCapture(t)()

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"404 Not Found"}`))
	})

	cfg := &config.AssignmentConfig{
		Course:   "mpd",
		Path:     "mpd/ss26/blatt-01",
		Per:      config.PerStudent,
		Branches: []config.BranchRule{{Name: "main", Protect: true}},
	}
	assertExitCode(t, 1, func() { client.ProtectToBranch(cfg) })
}

func TestProtectToBranch_InvalidPer_Exits(t *testing.T) {
	defer withExitCapture(t)()

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups" {
			_, _ = w.Write([]byte(`[{"id":1,"full_path":"mpd/ss26/blatt-01"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:   "mpd",
		Path:     "mpd/ss26/blatt-01",
		Per:      config.PerFailed,
		Branches: []config.BranchRule{{Name: "main", Protect: true}},
	}
	assertExitCode(t, 1, func() { client.ProtectToBranch(cfg) })
}

// ---- protectToBranchPerStudent ----------------------------------------------

func TestProtectToBranchPerStudent_NoStudents(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:   "mpd",
		Name:     "blatt01",
		Path:     "mpd/ss26/blatt-01",
		Per:      config.PerStudent,
		Students: []*config.Student{},
		Branches: []config.BranchRule{{Name: "main", Protect: true}},
	}
	client.protectToBranchPerStudent(cfg)
}

func TestProtectToBranchPerStudent_GetProjectFails(t *testing.T) {
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
		Branches:              []config.BranchRule{{Name: "main", Protect: true}},
	}
	// GetProject fails → prints error and returns
	client.protectToBranchPerStudent(cfg)
}

func TestProtectToBranchPerStudent_Success(t *testing.T) {
	pj := `{"id":1,"name":"mpd-blatt01-alice","path_with_namespace":"mpd/ss26/blatt-01/mpd-blatt01-alice","ssh_url_to_repo":"git@example.com:mpd/ss26/mpd-blatt01-alice.git"}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "mpd-blatt01-alice"):
			_, _ = w.Write([]byte(pj))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
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
		Branches:              []config.BranchRule{{Name: "main", Protect: true}},
	}
	client.protectToBranchPerStudent(cfg)
}

// ---- protectToBranchPerGroup ------------------------------------------------

func TestProtectToBranchPerGroup_NoGroups(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:   "mpd",
		Name:     "blatt01",
		Path:     "mpd/ss26/blatt-01",
		Per:      config.PerGroup,
		Groups:   []*config.Group{},
		Branches: []config.BranchRule{{Name: "main", Protect: true}},
	}
	client.protectToBranchPerGroup(cfg)
}

func TestProtectToBranchPerGroup_Success(t *testing.T) {
	pj := `{"id":2,"name":"mpd-blatt01-team1","path_with_namespace":"mpd/ss26/blatt-01/mpd-blatt01-team1","ssh_url_to_repo":"git@example.com:mpd/ss26/mpd-blatt01-team1.git"}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "mpd-blatt01-team1"):
			_, _ = w.Write([]byte(pj))
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
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
		Branches: []config.BranchRule{{Name: "main", Protect: true}},
	}
	client.protectToBranchPerGroup(cfg)
}

// ---- protectBranch ----------------------------------------------------------

func TestProtectBranch_NoFlags_IsNoOp(t *testing.T) {
	// No protected branches configured -> nothing happens
	called := false
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Branches: []config.BranchRule{{Name: "main", Protect: false, MergeOnly: false}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectBranch(cfg, project, false)
	if err != nil {
		t.Fatalf("protectBranch() (no-op) error = %v", err)
	}
	if called {
		t.Fatal("protectBranch() made HTTP calls when neither flag is set")
	}
}

func TestProtectBranch_ProtectToBranch_Success(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Branches: []config.BranchRule{{Name: "main", Protect: true}, {Name: "develop"}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectBranch(cfg, project, false)
	if err != nil {
		t.Fatalf("protectBranch(ProtectToBranch) error = %v", err)
	}
}

func TestProtectBranch_ProtectDevBranchMergeOnly_Success(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"develop"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Branches: []config.BranchRule{{Name: "main"}, {Name: "develop", MergeOnly: true}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectBranch(cfg, project, false)
	if err != nil {
		t.Fatalf("protectBranch(ProtectDevBranchMergeOnly) error = %v", err)
	}
}

func TestProtectBranch_BothSameBranch_Success(t *testing.T) {
	// If one branch is both protected and merge-only, merge-only semantics win.
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Branches: []config.BranchRule{{Name: "main", Protect: true, MergeOnly: true}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectBranch(cfg, project, false)
	if err != nil {
		t.Fatalf("protectBranch(both, same branch) error = %v", err)
	}
}

// TestProtectBranch_SendsAdditionalBranchProtectionFlags verifies that
// AllowForcePush and CodeOwnerApprovalRequired are forwarded to the GitLab API.
// The gitlab client-go library encodes POST/PATCH bodies as JSON
// (Content-Type: application/json), NOT as form-encoded values.
func TestProtectBranch_SendsAdditionalBranchProtectionFlags(t *testing.T) {
	var postBody map[string]any
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			if err := json.NewDecoder(r.Body).Decode(&postBody); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Branches: []config.BranchRule{{
			Name:                      "main",
			Protect:                   true,
			AllowForcePush:            true,
			CodeOwnerApprovalRequired: true,
		}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	if err := client.protectBranch(cfg, project, false); err != nil {
		t.Fatalf("protectBranch() error = %v", err)
	}
	if postBody["name"] != "main" {
		t.Errorf("name = %#v, want \"main\"", postBody["name"])
	}
	if postBody["allow_force_push"] != true {
		t.Errorf("allow_force_push = %#v, want true", postBody["allow_force_push"])
	}
	if postBody["code_owner_approval_required"] != true {
		t.Errorf("code_owner_approval_required = %#v, want true", postBody["code_owner_approval_required"])
	}
}

// TestProtectBranch_UpdateSendsAdditionalBranchProtectionFlags verifies that
// AllowForcePush and CodeOwnerApprovalRequired are also forwarded on the PATCH
// (update) path — i.e. when a protected-branch rule already exists.
func TestProtectBranch_UpdateSendsAdditionalBranchProtectionFlags(t *testing.T) {
	var patchBody map[string]any
	existingRule := `{"id":1,"name":"main","push_access_levels":[{"id":10,"access_level":40}],"merge_access_levels":[{"id":11,"access_level":40}],"unprotect_access_levels":[{"id":12,"access_level":40}]}`
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(existingRule))
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "protected_branches"):
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				t.Errorf("Content-Type = %q, want application/json", ct)
			}
			if err := json.NewDecoder(r.Body).Decode(&patchBody); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Branches: []config.BranchRule{{
			Name:                      "main",
			Protect:                   true,
			AllowForcePush:            true,
			CodeOwnerApprovalRequired: true,
		}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	if err := client.protectBranch(cfg, project, false); err != nil {
		t.Fatalf("protectBranch() (update path) error = %v", err)
	}
	if patchBody["allow_force_push"] != true {
		t.Errorf("allow_force_push = %#v, want true", patchBody["allow_force_push"])
	}
	if patchBody["code_owner_approval_required"] != true {
		t.Errorf("code_owner_approval_required = %#v, want true", patchBody["code_owner_approval_required"])
	}
}

// ---- protectSingleBranch ----------------------------------------------------

func TestProtectSingleBranch_Success(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"main","push_access_levels":[{"id":10,"access_level":40}],"merge_access_levels":[{"id":11,"access_level":40}],"unprotect_access_levels":[{"id":12,"access_level":40}]}`))
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectSingleBranch(project, config.BranchRule{Name: "main"}, gitlabapi.MaintainerPermissions, gitlabapi.MaintainerPermissions)
	if err != nil {
		t.Fatalf("protectSingleBranch() error = %v", err)
	}
}

func TestProtectSingleBranch_MergeOnly_RecreatesExistingRule(t *testing.T) {
	deleteCalled := false
	postCalled := false
	patchCalled := false

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"main","push_access_levels":[{"id":10,"access_level":40}],"merge_access_levels":[{"id":11,"access_level":40}],"unprotect_access_levels":[{"id":12,"access_level":40}]}`))
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "protected_branches"):
			deleteCalled = true
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			postCalled = true
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "protected_branches"):
			patchCalled = true
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectSingleBranch(project, config.BranchRule{Name: "main", MergeOnly: true}, gitlabapi.NoPermissions, gitlabapi.DeveloperPermissions)
	if err != nil {
		t.Fatalf("protectSingleBranch() error = %v", err)
	}
	if !deleteCalled {
		t.Fatal("protectSingleBranch() did not unprotect existing branch before recreating merge-only rule")
	}
	if !postCalled {
		t.Fatal("protectSingleBranch() did not create merge-only protected branch rule")
	}
	if patchCalled {
		t.Fatal("protectSingleBranch() should not patch existing protected branch for merge-only rule")
	}
}

func TestProtectSingleBranch_GetNotFound_ProtectStillCalled(t *testing.T) {
	// Get returns 404 (branch not yet protected) -> protectSingleBranch creates the rule.
	protectCalled := false
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNotFound) // not protected yet
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			protectCalled = true
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectSingleBranch(project, config.BranchRule{Name: "main"}, gitlabapi.NoPermissions, gitlabapi.DeveloperPermissions)
	if err != nil {
		t.Fatalf("protectSingleBranch() error = %v", err)
	}
	if !protectCalled {
		t.Fatal("protectSingleBranch() did not call ProtectRepositoryBranches")
	}
}

func TestProtectSingleBranch_ProtectFails_ReturnsError(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectSingleBranch(project, config.BranchRule{Name: "main"}, gitlabapi.MaintainerPermissions, gitlabapi.MaintainerPermissions)
	if err == nil {
		t.Fatal("protectSingleBranch() expected error on 403, got nil")
	}
}
