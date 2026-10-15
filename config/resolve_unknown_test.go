package config

import (
	"slices"
	"testing"
)

// Resolution records ignored keys so the CLI can warn about them on every
// command, not only under `glabs config lint`.
func TestResolveAssignmentRecordsUnknownKeys(t *testing.T) {
	t.Parallel()

	cfg, err := ResolveAssignmentFromBytes([]byte(`
demo:
  coursepath: demo/semester
  students:
    - alice
  blatt1:
    assignmentpath: blatt-1
    per: student
    branches:
      - name: main
        protectToBranch: true
`), "demo", "blatt1", Globals{GitlabHost: "https://gitlab.example.org"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	want := []string{"demo.blatt1.branches[0].protectToBranch"}
	if !slices.Equal(cfg.UnknownKeys, want) {
		t.Errorf("UnknownKeys = %v, want %v", cfg.UnknownKeys, want)
	}
	if len(cfg.Branches) != 1 || cfg.Branches[0].Protect {
		t.Errorf("branches = %+v, want main unprotected", cfg.Branches)
	}
}

func TestResolveAssignmentHasNoUnknownKeysForCleanConfig(t *testing.T) {
	t.Parallel()

	cfg, err := ResolveAssignmentFromBytes([]byte(`
demo:
  coursepath: demo/semester
  students:
    - alice
  base:
    abstract: true
    per: student
  blatt1:
    extends: base
    assignmentpath: blatt-1
    branches:
      - name: main
        mergeOnly: true
`), "demo", "blatt1", Globals{GitlabHost: "https://gitlab.example.org"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(cfg.UnknownKeys) != 0 {
		t.Errorf("UnknownKeys = %v, want none", cfg.UnknownKeys)
	}
}
