package config

import "testing"

func TestSetBranch(t *testing.T) {
	cfg := &AssignmentConfig{Clone: &Clone{Branch: "main"}}
	cfg.SetBranch("develop")
	if cfg.Clone.Branch != "develop" {
		t.Fatalf("SetBranch() = %q, want %q", cfg.Clone.Branch, "develop")
	}
}

func TestSetBranch_Empty(t *testing.T) {
	cfg := &AssignmentConfig{Clone: &Clone{Branch: "main"}}
	cfg.SetBranch("")
	if cfg.Clone.Branch != "" {
		t.Fatalf("SetBranch(\"\") = %q, want empty", cfg.Clone.Branch)
	}
}

func TestSetProtectToBranch_WithBranch(t *testing.T) {
	cfg := &AssignmentConfig{}
	cfg.SetProtectToBranch("feature")
	if len(cfg.Branches) != 1 {
		t.Fatalf("len(Branches) = %d, want 1", len(cfg.Branches))
	}
	if cfg.Branches[0].Name != "feature" || !cfg.Branches[0].Protect {
		t.Fatalf("branch config = %#v", cfg.Branches[0])
	}
}

func TestSetProtectToBranch_EmptyBranch(t *testing.T) {
	cfg := &AssignmentConfig{Branches: []BranchRule{{Name: "main"}}}
	cfg.SetProtectToBranch("")
	if len(cfg.Branches) != 1 {
		t.Fatalf("len(Branches) = %d, want 1", len(cfg.Branches))
	}
	if cfg.Branches[0].Name != "main" || !cfg.Branches[0].Protect {
		t.Fatalf("branch config = %#v", cfg.Branches[0])
	}
}

func TestSetLocalpath(t *testing.T) {
	cfg := &AssignmentConfig{Clone: &Clone{LocalPath: ""}}
	cfg.SetLocalpath("/tmp/repos")
	if cfg.Clone.LocalPath != "/tmp/repos" {
		t.Fatalf("LocalPath = %q, want /tmp/repos", cfg.Clone.LocalPath)
	}
}

func TestSetLocalpath_Override(t *testing.T) {
	cfg := &AssignmentConfig{Clone: &Clone{LocalPath: "/old"}}
	cfg.SetLocalpath("/new")
	if cfg.Clone.LocalPath != "/new" {
		t.Fatalf("LocalPath = %q, want /new", cfg.Clone.LocalPath)
	}
}

func TestSetForce(t *testing.T) {
	cfg := &AssignmentConfig{Clone: &Clone{Force: false}}
	cfg.SetForce()
	if !cfg.Clone.Force {
		t.Fatal("Force should be true after SetForce()")
	}
}

func TestSetForce_Idempotent(t *testing.T) {
	cfg := &AssignmentConfig{Clone: &Clone{Force: true}}
	cfg.SetForce()
	if !cfg.Clone.Force {
		t.Fatal("Force should remain true after second SetForce()")
	}
}
