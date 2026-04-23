package gitlab

import (
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
		Course: "mpd",
		Path:   "mpd/ss26/blatt-01",
		Per:    config.PerStudent,
		Startercode: &config.Startercode{
			ToBranch:        "main",
			ProtectToBranch: true,
		},
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
		Course: "mpd",
		Path:   "mpd/ss26/blatt-01",
		Per:    config.PerFailed,
		Startercode: &config.Startercode{
			ToBranch:        "main",
			ProtectToBranch: true,
		},
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
		Startercode: &config.Startercode{
			ToBranch:        "main",
			ProtectToBranch: true,
		},
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
		Startercode: &config.Startercode{
			ToBranch:        "main",
			ProtectToBranch: true,
		},
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
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNoContent)
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
		Startercode: &config.Startercode{
			ToBranch:        "main",
			ProtectToBranch: true,
		},
	}
	client.protectToBranchPerStudent(cfg)
}

// ---- protectToBranchPerGroup ------------------------------------------------

func TestProtectToBranchPerGroup_NoGroups(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Name:   "blatt01",
		Path:   "mpd/ss26/blatt-01",
		Per:    config.PerGroup,
		Groups: []*config.Group{},
		Startercode: &config.Startercode{
			ToBranch:        "main",
			ProtectToBranch: true,
		},
	}
	client.protectToBranchPerGroup(cfg)
}

func TestProtectToBranchPerGroup_Success(t *testing.T) {
	pj := `{"id":2,"name":"mpd-blatt01-team1","path_with_namespace":"mpd/ss26/blatt-01/mpd-blatt01-team1","ssh_url_to_repo":"git@example.com:mpd/ss26/mpd-blatt01-team1.git"}`

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "mpd-blatt01-team1"):
			_, _ = w.Write([]byte(pj))
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNoContent)
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
		Startercode: &config.Startercode{
			ToBranch:        "main",
			ProtectToBranch: true,
		},
	}
	client.protectToBranchPerGroup(cfg)
}

// ---- protectBranch ----------------------------------------------------------

func TestProtectBranch_NoFlags_IsNoOp(t *testing.T) {
	// Neither ProtectToBranch nor ProtectDevBranchMergeOnly → nothing happens
	called := false
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{
			ToBranch:                  "main",
			ProtectToBranch:           false,
			ProtectDevBranchMergeOnly: false,
		},
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
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{
			ToBranch:        "main",
			DevBranch:       "develop",
			ProtectToBranch: true,
		},
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
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"develop"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{
			ToBranch:                  "main",
			DevBranch:                 "develop",
			ProtectDevBranchMergeOnly: true,
		},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectBranch(cfg, project, false)
	if err != nil {
		t.Fatalf("protectBranch(ProtectDevBranchMergeOnly) error = %v", err)
	}
}

func TestProtectBranch_BothSameBranch_Success(t *testing.T) {
	// ProtectDevBranchMergeOnly=true AND DevBranch==ToBranch → single protectSingleBranch call
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	cfg := &config.AssignmentConfig{
		Startercode: &config.Startercode{
			ToBranch:                  "main",
			DevBranch:                 "main", // same as ToBranch
			ProtectToBranch:           true,
			ProtectDevBranchMergeOnly: true,
		},
	}
	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectBranch(cfg, project, false)
	if err != nil {
		t.Fatalf("protectBranch(both, same branch) error = %v", err)
	}
}

// ---- protectSingleBranch ----------------------------------------------------

func TestProtectSingleBranch_Success(t *testing.T) {
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectSingleBranch(project, "main", gitlabapi.MaintainerPermissions, gitlabapi.MaintainerPermissions)
	if err != nil {
		t.Fatalf("protectSingleBranch() error = %v", err)
	}
}

func TestProtectSingleBranch_UnprotectFails_ProtectStillCalled(t *testing.T) {
	// Unprotect returns 404 (branch not yet protected) → protectSingleBranch continues
	protectCalled := false
	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNotFound) // not protected yet
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			protectCalled = true
			_, _ = w.Write([]byte(`{"id":1,"name":"main"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectSingleBranch(project, "main", gitlabapi.NoPermissions, gitlabapi.DeveloperPermissions)
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
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "protected_branches"):
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"403 Forbidden"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	project := &gitlabapi.Project{ID: 1, Name: "myrepo"}
	err := client.protectSingleBranch(project, "main", gitlabapi.MaintainerPermissions, gitlabapi.MaintainerPermissions)
	if err == nil {
		t.Fatal("protectSingleBranch() expected error on 403, got nil")
	}
}
