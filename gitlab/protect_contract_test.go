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

func TestProtectBranch_AppliesMergeRequestApprovals(t *testing.T) {
	var createBody map[string]any
	protectedMain := `{"id":10,"name":"main","push_access_levels":[{"id":10,"access_level":40}],"merge_access_levels":[{"id":11,"access_level":40}],"unprotect_access_levels":[{"id":12,"access_level":40}]}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups":
			if r.URL.Query().Get("search") != "tutors" {
				t.Fatalf("groups search query = %q, want tutors", r.URL.Query().Get("search"))
			}
			_, _ = w.Write([]byte(`[{"id":55,"full_path":"mpd/tutors"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/protected_branches/main":
			_, _ = w.Write([]byte(protectedMain))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v4/projects/1/protected_branches/main":
			_, _ = w.Write([]byte(`{"id":10,"name":"main"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/protected_branches":
			_, _ = w.Write([]byte(`[{"id":10,"name":"main"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/approval_rules":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/1/approval_rules":
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}
			_, _ = w.Write([]byte(`{"id":77,"name":"review-main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Branches: []config.BranchRule{{Name: "main", Protect: true}},
		MergeRequest: &config.MergeRequest{Approvals: []config.MergeRequestApprovalRule{{
			Name:              "review-main",
			Branches:          []string{"main"},
			Usernames:         []string{"@me"},
			Groups:            []string{"mpd/tutors"},
			RequiredApprovals: 2,
		}}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	if err := client.protectBranch(cfg, project, false); err != nil {
		t.Fatalf("protectBranch() error = %v", err)
	}

	if createBody["name"] != "review-main" {
		t.Fatalf("approval rule name = %#v, want review-main", createBody["name"])
	}
	if createBody["approvals_required"] != float64(2) {
		t.Fatalf("approvals_required = %#v, want 2", createBody["approvals_required"])
	}

	usernames, ok := createBody["usernames"].([]any)
	if !ok || len(usernames) != 1 || usernames[0] != "me" {
		t.Fatalf("usernames = %#v, want [\"me\"]", createBody["usernames"])
	}
	groupIDs, ok := createBody["group_ids"].([]any)
	if !ok || len(groupIDs) != 1 || groupIDs[0] != float64(55) {
		t.Fatalf("group_ids = %#v, want [55]", createBody["group_ids"])
	}
	branchIDs, ok := createBody["protected_branch_ids"].([]any)
	if !ok || len(branchIDs) != 1 || branchIDs[0] != float64(10) {
		t.Fatalf("protected_branch_ids = %#v, want [10]", createBody["protected_branch_ids"])
	}
}

func TestProtectBranch_AppliesMergeRequestApprovalSettings(t *testing.T) {
	var body map[string]any
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/1/approvals":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}
			_, _ = w.Write([]byte(`{"merge_requests_author_approval":false}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	preventCreator := true
	preventCommitters := true
	preventEditing := true
	reauth := true
	whenCommitAdded := config.ApprovalRemoveCodeOwnerApprovalsIfFilesChanged
	cfg := &config.AssignmentConfig{
		MergeRequest: &config.MergeRequest{
			ApprovalSettings: &config.MergeRequestApprovalSettings{
				PreventApprovalByMergeRequestCreator:       &preventCreator,
				PreventApprovalsByUsersWhoAddCommits:       &preventCommitters,
				PreventEditingApprovalRulesInMergeRequests: &preventEditing,
				RequireUserReauthenticationToApprove:       &reauth,
				WhenCommitAdded:                            &whenCommitAdded,
			},
		},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	if err := client.protectBranch(cfg, project, false); err != nil {
		t.Fatalf("protectBranch() error = %v", err)
	}

	if body["merge_requests_author_approval"] != false {
		t.Fatalf("merge_requests_author_approval = %#v, want false", body["merge_requests_author_approval"])
	}
	if body["merge_requests_disable_committers_approval"] != true {
		t.Fatalf("merge_requests_disable_committers_approval = %#v, want true", body["merge_requests_disable_committers_approval"])
	}
	if body["disable_overriding_approvers_per_merge_request"] != true {
		t.Fatalf("disable_overriding_approvers_per_merge_request = %#v, want true", body["disable_overriding_approvers_per_merge_request"])
	}
	if body["require_reauthentication_to_approve"] != true {
		t.Fatalf("require_reauthentication_to_approve = %#v, want true", body["require_reauthentication_to_approve"])
	}
	if body["reset_approvals_on_push"] != false {
		t.Fatalf("reset_approvals_on_push = %#v, want false", body["reset_approvals_on_push"])
	}
	if body["selective_code_owner_removals"] != true {
		t.Fatalf("selective_code_owner_removals = %#v, want true", body["selective_code_owner_removals"])
	}
}

func TestProtectBranch_ClearsApprovalRuleWhenNoApproversConfigured(t *testing.T) {
	deleted := false
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/protected_branches":
			_, _ = w.Write([]byte(`[{"id":10,"name":"main"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/approval_rules":
			_, _ = w.Write([]byte(`[{"id":9,"name":"no-review"}]`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v4/projects/1/approval_rules/9":
			deleted = true
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	cfg := &config.AssignmentConfig{
		MergeRequest: &config.MergeRequest{Approvals: []config.MergeRequestApprovalRule{{
			Name:              "no-review",
			Branches:          []string{"main"},
			RequiredApprovals: 0,
		}}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	if err := client.protectBranch(cfg, project, false); err != nil {
		t.Fatalf("protectBranch() error = %v", err)
	}
	if !deleted {
		t.Fatal("approval rule delete was not called")
	}
}

func TestProtectBranch_SkipsApprovalRuleForUnprotectedBranch(t *testing.T) {
	created := 0
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/protected_branches":
			_, _ = w.Write([]byte(`[{"id":10,"name":"main"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/approval_rules":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/1/approval_rules":
			created++
			_, _ = w.Write([]byte(`{"id":77,"name":"main-review"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		MergeRequest: &config.MergeRequest{Approvals: []config.MergeRequestApprovalRule{
			{
				Name:              "main-review",
				Branches:          []string{"main"},
				Usernames:         []string{"me"},
				RequiredApprovals: 1,
			},
			{
				Name:              "missing-branch-review",
				Branches:          []string{"blubber"},
				Usernames:         []string{"me"},
				RequiredApprovals: 1,
			},
		}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	if err := client.protectBranch(cfg, project, false); err != nil {
		t.Fatalf("protectBranch() error = %v", err)
	}
	if created != 1 {
		t.Fatalf("created approval rules = %d, want 1", created)
	}
}

func TestApplyMergeRequestApprovalRules_MergesAnyApproverRulesAcrossBranches(t *testing.T) {
	var createBody map[string]any
	created := 0
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/protected_branches":
			_, _ = w.Write([]byte(`[{"id":10,"name":"main"},{"id":11,"name":"blubber"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/approval_rules":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/1/approval_rules":
			created++
			if err := json.NewDecoder(r.Body).Decode(&createBody); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}
			_, _ = w.Write([]byte(`{"id":77,"name":"main-review","rule_type":"any_approver"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		MergeRequest: &config.MergeRequest{Approvals: []config.MergeRequestApprovalRule{
			{Name: "main-review", Branches: []string{"main"}, RequiredApprovals: 1},
			{Name: "blubber-review", Branches: []string{"blubber"}, RequiredApprovals: 1},
		}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	if err := client.applyMergeRequestApprovalRules(cfg, project); err != nil {
		t.Fatalf("applyMergeRequestApprovalRules() error = %v", err)
	}
	if created != 1 {
		t.Fatalf("created approval rules = %d, want 1", created)
	}
	if createBody["rule_type"] != "any_approver" {
		t.Fatalf("rule_type = %#v, want any_approver", createBody["rule_type"])
	}
	if createBody["approvals_required"] != float64(1) {
		t.Fatalf("approvals_required = %#v, want 1", createBody["approvals_required"])
	}
	branchIDs, ok := createBody["protected_branch_ids"].([]any)
	if !ok || len(branchIDs) != 2 || branchIDs[0] != float64(11) || branchIDs[1] != float64(10) {
		t.Fatalf("protected_branch_ids = %#v, want [11 10]", createBody["protected_branch_ids"])
	}
}

func TestApplyMergeRequestApprovalRules_UpdatesExistingAnyApproverRule(t *testing.T) {
	var updateBody map[string]any
	updated := 0
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/protected_branches":
			_, _ = w.Write([]byte(`[{"id":10,"name":"main"},{"id":11,"name":"blubber"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/approval_rules":
			_, _ = w.Write([]byte(`[{"id":9,"name":"Minimum required approvals","rule_type":"any_approver"}]`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v4/projects/1/approval_rules/9":
			updated++
			if err := json.NewDecoder(r.Body).Decode(&updateBody); err != nil {
				t.Fatalf("json.Decode() error = %v", err)
			}
			_, _ = w.Write([]byte(`{"id":9,"name":"Minimum required approvals","rule_type":"any_approver"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		MergeRequest: &config.MergeRequest{Approvals: []config.MergeRequestApprovalRule{
			{Name: "main-review", Branches: []string{"main"}, RequiredApprovals: 1},
			{Name: "blubber-review", Branches: []string{"blubber"}, RequiredApprovals: 1},
		}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	if err := client.applyMergeRequestApprovalRules(cfg, project); err != nil {
		t.Fatalf("applyMergeRequestApprovalRules() error = %v", err)
	}
	if updated != 1 {
		t.Fatalf("updated approval rules = %d, want 1", updated)
	}
	if updateBody["approvals_required"] != float64(1) {
		t.Fatalf("approvals_required = %#v, want 1", updateBody["approvals_required"])
	}
	branchIDs, ok := updateBody["protected_branch_ids"].([]any)
	if !ok || len(branchIDs) != 2 || branchIDs[0] != float64(11) || branchIDs[1] != float64(10) {
		t.Fatalf("protected_branch_ids = %#v, want [11 10]", updateBody["protected_branch_ids"])
	}
}

func TestApplyMergeRequestApprovalRules_RejectsMultipleAnyApproverApprovalCounts(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/protected_branches":
			_, _ = w.Write([]byte(`[{"id":10,"name":"main"},{"id":11,"name":"blubber"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/approval_rules":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		MergeRequest: &config.MergeRequest{Approvals: []config.MergeRequestApprovalRule{
			{Name: "main-review", Branches: []string{"main"}, RequiredApprovals: 1},
			{Name: "blubber-review", Branches: []string{"blubber"}, RequiredApprovals: 2},
		}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.applyMergeRequestApprovalRules(cfg, project)
	if err == nil {
		t.Fatal("applyMergeRequestApprovalRules() expected error, got nil")
	}
	if !strings.Contains(err.Error(), "only one any-approver rule") {
		t.Fatalf("error = %q, want any-approver guidance", err)
	}
}

func TestApplyMergeRequestApprovalRules_MultiMemberGroupsOnly_SkipsForStudentProjects(t *testing.T) {
	created := 0
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/protected_branches":
			_, _ = w.Write([]byte(`[{"id":10,"name":"main"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/approval_rules":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/1/approval_rules":
			created++
			_, _ = w.Write([]byte(`{"id":77,"name":"review-main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Per: config.PerStudent,
		MergeRequest: &config.MergeRequest{Approvals: []config.MergeRequestApprovalRule{{
			Name:                  "review-main",
			Branches:              []string{"main"},
			Usernames:             []string{"me"},
			MultiMemberGroupsOnly: true,
			RequiredApprovals:     1,
		}}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	if err := client.applyMergeRequestApprovalRulesForMemberCount(cfg, project, 1); err != nil {
		t.Fatalf("applyMergeRequestApprovalRulesForMemberCount() error = %v", err)
	}
	if created != 0 {
		t.Fatalf("created approval rules = %d, want 0", created)
	}
}

func TestApplyMergeRequestApprovalRules_MultiMemberGroupsOnly_SkipsForSingleMemberGroup(t *testing.T) {
	created := 0
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/protected_branches":
			_, _ = w.Write([]byte(`[{"id":10,"name":"main"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/approval_rules":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/1/approval_rules":
			created++
			_, _ = w.Write([]byte(`{"id":77,"name":"review-main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Per: config.PerGroup,
		MergeRequest: &config.MergeRequest{Approvals: []config.MergeRequestApprovalRule{{
			Name:                  "review-main",
			Branches:              []string{"main"},
			Usernames:             []string{"me"},
			MultiMemberGroupsOnly: true,
			RequiredApprovals:     1,
		}}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	if err := client.applyMergeRequestApprovalRulesForMemberCount(cfg, project, 1); err != nil {
		t.Fatalf("applyMergeRequestApprovalRulesForMemberCount() error = %v", err)
	}
	if created != 0 {
		t.Fatalf("created approval rules = %d, want 0", created)
	}
}

func TestApplyMergeRequestApprovalRules_MultiMemberGroupsOnly_AppliesForMultiMemberGroup(t *testing.T) {
	created := 0
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/protected_branches":
			_, _ = w.Write([]byte(`[{"id":10,"name":"main"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/approval_rules":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v4/projects/1/approval_rules":
			created++
			_, _ = w.Write([]byte(`{"id":77,"name":"review-main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Per: config.PerGroup,
		MergeRequest: &config.MergeRequest{Approvals: []config.MergeRequestApprovalRule{{
			Name:                  "review-main",
			Branches:              []string{"main"},
			Usernames:             []string{"me"},
			MultiMemberGroupsOnly: true,
			RequiredApprovals:     1,
		}}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	if err := client.applyMergeRequestApprovalRulesForMemberCount(cfg, project, 2); err != nil {
		t.Fatalf("applyMergeRequestApprovalRulesForMemberCount() error = %v", err)
	}
	if created != 1 {
		t.Fatalf("created approval rules = %d, want 1", created)
	}
}

func TestProtectBranch_EmailApproverNotSupported_ReturnsError(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/protected_branches":
			_, _ = w.Write([]byte(`[{"id":10,"name":"main"}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v4/projects/1/approval_rules":
			_, _ = w.Write([]byte(`[]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		MergeRequest: &config.MergeRequest{Approvals: []config.MergeRequestApprovalRule{{
			Name:              "main-review",
			Branches:          []string{"main"},
			Usernames:         []string{"me@example.org"},
			RequiredApprovals: 1,
		}}},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectBranch(cfg, project, false)
	if err == nil {
		t.Fatal("protectBranch() expected error for email approver, got nil")
	}
	if !strings.Contains(err.Error(), "email") || !strings.Contains(err.Error(), "use username") {
		t.Fatalf("error = %q, want email guidance", err)
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
