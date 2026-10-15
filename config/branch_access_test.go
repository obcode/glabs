package config

import (
	"strings"
	"testing"
)

func TestBranchRuleAccessLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		rule       BranchRule
		protected  bool
		push, merg BranchAccess
	}{
		{"unprotected", BranchRule{Name: "main"}, false, BranchAccessMaintainer, BranchAccessMaintainer},
		{"protect", BranchRule{Protect: true}, true, BranchAccessMaintainer, BranchAccessMaintainer},
		{"mergeOnly", BranchRule{MergeOnly: true}, true, BranchAccessNoOne, BranchAccessDeveloper},
		{"mergeOnly wins over protect", BranchRule{Protect: true, MergeOnly: true}, true, BranchAccessNoOne, BranchAccessDeveloper},
		{"locked", BranchRule{Push: "noOne", Merge: "noOne"}, true, BranchAccessNoOne, BranchAccessNoOne},
		{"merge overrides mergeOnly half", BranchRule{MergeOnly: true, Merge: "maintainer"}, true, BranchAccessNoOne, BranchAccessMaintainer},
		{"push alone", BranchRule{Push: "developer"}, true, BranchAccessDeveloper, BranchAccessMaintainer},
	}
	for _, tt := range tests {
		if got := tt.rule.Protected(); got != tt.protected {
			t.Errorf("%s: Protected() = %v, want %v", tt.name, got, tt.protected)
		}
		push, merge := tt.rule.AccessLevels()
		if push != tt.push || merge != tt.merg {
			t.Errorf("%s: AccessLevels() = %s/%s, want %s/%s", tt.name, push, merge, tt.push, tt.merg)
		}
	}
}

func TestResolveBranchesCanonicalisesAccessLevels(t *testing.T) {
	t.Parallel()

	cfg, err := ResolveAssignmentFromBytes([]byte(`
demo:
  coursepath: demo/semester
  blatt1:
    per: student
    students: [alice]
    branches:
      - name: main
        push: NoOne
        merge: noone
      - name: main
        merge: Developer
`), "demo", "blatt1", Globals{GitlabHost: "https://gitlab.example.org"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(cfg.Branches) != 1 {
		t.Fatalf("branches = %+v, want the duplicate merged into one", cfg.Branches)
	}
	if b := cfg.Branches[0]; b.Push != "noOne" || b.Merge != "developer" {
		t.Errorf("push/merge = %q/%q, want noOne/developer (later entry overrides merge)", b.Push, b.Merge)
	}
}

func TestResolveBranchesRejectsUnknownAccessLevel(t *testing.T) {
	t.Parallel()

	_, err := ResolveAssignmentFromBytes([]byte(`
demo:
  coursepath: demo/semester
  blatt1:
    per: student
    students: [alice]
    branches:
      - name: main
        push: nobody
`), "demo", "blatt1", Globals{GitlabHost: "https://gitlab.example.org"})
	if err == nil {
		t.Fatal("resolve succeeded, want an error for push: nobody")
	}
	if !strings.Contains(err.Error(), "branches[0].push") || !strings.Contains(err.Error(), "noOne") {
		t.Errorf("error %q does not name the key and the valid values", err)
	}
}
