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
	cfg := &AssignmentConfig{Startercode: &Startercode{ToBranch: "main"}}
	cfg.SetProtectToBranch("feature")
	if cfg.Startercode.ToBranch != "feature" {
		t.Fatalf("ToBranch = %q, want %q", cfg.Startercode.ToBranch, "feature")
	}
	if !cfg.Startercode.ProtectToBranch {
		t.Fatal("ProtectToBranch should be true")
	}
}

func TestSetProtectToBranch_EmptyBranch(t *testing.T) {
	cfg := &AssignmentConfig{Startercode: &Startercode{ToBranch: "main"}}
	cfg.SetProtectToBranch("")
	// empty: ToBranch stays unchanged, ProtectToBranch is set to true
	if cfg.Startercode.ToBranch != "main" {
		t.Fatalf("ToBranch = %q, want %q", cfg.Startercode.ToBranch, "main")
	}
	if !cfg.Startercode.ProtectToBranch {
		t.Fatal("ProtectToBranch should be true even with empty branch string")
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
