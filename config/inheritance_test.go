package config

import (
	"testing"

	"github.com/spf13/viper"
)

// baseAssignment sets up a fully-featured parent assignment ("blatt09") that
// children inherit from.
func baseAssignment(t *testing.T) {
	t.Helper()
	resetViper(t)

	viper.Set("gitlab.host", "https://gitlab.example.org")
	viper.Set("mpd", true)
	viper.Set("mpd.coursepath", "mpd")
	viper.Set("mpd.semesterpath", "ss26")
	viper.Set("mpd.useCoursenameAsPrefix", true)
	viper.Set("mpd.students", []string{"alice"})

	viper.Set("mpd.blatt09", true)
	viper.Set("mpd.blatt09.assignmentpath", "blatt-09")
	viper.Set("mpd.blatt09.description", "Blatt 9")
	viper.Set("mpd.blatt09.per", "student")
	viper.Set("mpd.blatt09.mergeRequest", map[string]any{
		"mergeMethod":  "semi_linear",
		"squashOption": "never",
		"pipeline":     true,
	})
	viper.Set("mpd.blatt09.startercode", map[string]any{
		"url":        "git@gitlab.lrz.de:mpd/labs/blatt-09.git",
		"fromBranch": "startercode",
		"template":   true,
	})
	viper.Set("mpd.blatt09.branches", []map[string]any{
		{"name": "main", "mergeOnly": true},
	})
	viper.Set("mpd.blatt09.deferredBranches", map[string]any{
		"devcontainer": map[string]any{
			"url":        "git@gitlab.lrz.de:mpd/devcontainer.git",
			"fromBranch": "main",
			"toBranch":   "devcontainer",
		},
		"solution": map[string]any{
			"fromBranch":    "solution",
			"orphan":        true,
			"orphanMessage": "Lösung 9",
		},
	})
}

func TestInheritance_OverridesAndInherits(t *testing.T) {
	baseAssignment(t)

	// blatt10 extends blatt09 and only overrides what differs.
	viper.Set("mpd.blatt10", true)
	viper.Set("mpd.blatt10.extends", "blatt09")
	viper.Set("mpd.blatt10.assignmentpath", "blatt-10")
	viper.Set("mpd.blatt10.description", "Blatt 10")
	// override only one nested field of startercode
	viper.Set("mpd.blatt10.startercode", map[string]any{
		"url": "git@gitlab.lrz.de:mpd/labs/blatt-10.git",
	})

	cfg := mustAssignmentConfig(t, "mpd", "blatt10")

	// overridden scalars
	if cfg.Path != "mpd/ss26/blatt-10" {
		t.Fatalf("Path = %q, want mpd/ss26/blatt-10", cfg.Path)
	}
	if cfg.Description != "Blatt 10" {
		t.Fatalf("Description = %q, want Blatt 10", cfg.Description)
	}

	// inherited mergeRequest
	if cfg.MergeRequest == nil || cfg.MergeRequest.MergeMethod != SemiLinearHistory {
		t.Fatalf("MergeRequest = %#v, want inherited semi_linear", cfg.MergeRequest)
	}
	if !cfg.MergeRequest.PipelineMustSucceed {
		t.Fatal("PipelineMustSucceed should be inherited as true")
	}

	// startercode: url overridden, rest deep-merged from parent
	if cfg.Startercode == nil {
		t.Fatal("Startercode should not be nil")
	}
	if cfg.Startercode.URL != "git@gitlab.lrz.de:mpd/labs/blatt-10.git" {
		t.Fatalf("Startercode.URL = %q, want overridden blatt-10 url", cfg.Startercode.URL)
	}
	if cfg.Startercode.FromBranch != "startercode" {
		t.Fatalf("Startercode.FromBranch = %q, want inherited 'startercode'", cfg.Startercode.FromBranch)
	}
	if !cfg.Startercode.Template {
		t.Fatal("Startercode.Template should be inherited as true")
	}

	// deferredBranches inherited
	if len(cfg.DeferredBranches) != 2 {
		t.Fatalf("DeferredBranches len = %d, want 2", len(cfg.DeferredBranches))
	}
	if db, ok := cfg.DeferredBranches["solution"]; !ok || db.OrphanMessage != "Lösung 9" {
		t.Fatalf("inherited solution deferred branch = %#v", db)
	}

	// branches inherited
	if len(cfg.Branches) != 1 || cfg.Branches[0].Name != "main" || !cfg.Branches[0].MergeOnly {
		t.Fatalf("Branches = %#v, want inherited [main mergeOnly]", cfg.Branches)
	}
}

func TestInheritance_DeepMergeNestedDeferredBranch(t *testing.T) {
	baseAssignment(t)

	viper.Set("mpd.blatt10", true)
	viper.Set("mpd.blatt10.extends", "blatt09")
	viper.Set("mpd.blatt10.assignmentpath", "blatt-10")
	// override only the orphanMessage of the inherited "solution" deferred branch
	viper.Set("mpd.blatt10.deferredBranches", map[string]any{
		"solution": map[string]any{
			"orphanMessage": "Lösung 10",
		},
	})

	cfg := mustAssignmentConfig(t, "mpd", "blatt10")

	if len(cfg.DeferredBranches) != 2 {
		t.Fatalf("DeferredBranches len = %d, want 2 (devcontainer kept)", len(cfg.DeferredBranches))
	}
	sol, ok := cfg.DeferredBranches["solution"]
	if !ok {
		t.Fatal("solution deferred branch missing")
	}
	if sol.OrphanMessage != "Lösung 10" {
		t.Fatalf("solution.OrphanMessage = %q, want overridden 'Lösung 10'", sol.OrphanMessage)
	}
	if sol.FromBranch != "solution" {
		t.Fatalf("solution.FromBranch = %q, want inherited 'solution'", sol.FromBranch)
	}
	if _, ok := cfg.DeferredBranches["devcontainer"]; !ok {
		t.Fatal("devcontainer deferred branch should be inherited")
	}
}

func TestInheritance_MultiLevelChain(t *testing.T) {
	baseAssignment(t)

	viper.Set("mpd.blatt10", true)
	viper.Set("mpd.blatt10.extends", "blatt09")
	viper.Set("mpd.blatt10.assignmentpath", "blatt-10")

	viper.Set("mpd.blatt11", true)
	viper.Set("mpd.blatt11.extends", "blatt10")
	viper.Set("mpd.blatt11.assignmentpath", "blatt-11")

	cfg := mustAssignmentConfig(t, "mpd", "blatt11")

	if cfg.Path != "mpd/ss26/blatt-11" {
		t.Fatalf("Path = %q, want mpd/ss26/blatt-11", cfg.Path)
	}
	// inherited transitively from blatt09 via blatt10
	if cfg.MergeRequest == nil || cfg.MergeRequest.MergeMethod != SemiLinearHistory {
		t.Fatalf("MergeRequest = %#v, want transitively inherited semi_linear", cfg.MergeRequest)
	}
	if cfg.Startercode == nil || cfg.Startercode.FromBranch != "startercode" {
		t.Fatalf("Startercode = %#v, want transitively inherited", cfg.Startercode)
	}
}

func TestInheritance_NoExtendsIsUnaffected(t *testing.T) {
	baseAssignment(t)

	cfg := mustAssignmentConfig(t, "mpd", "blatt09")

	if cfg.Path != "mpd/ss26/blatt-09" {
		t.Fatalf("Path = %q", cfg.Path)
	}
	if cfg.Startercode == nil || cfg.Startercode.URL != "git@gitlab.lrz.de:mpd/labs/blatt-09.git" {
		t.Fatalf("Startercode = %#v", cfg.Startercode)
	}
}

func TestAssignmentIsAbstract(t *testing.T) {
	baseAssignment(t)

	// blatt09 (from baseAssignment) is concrete.
	if assignmentIsAbstract("mpd", "blatt09") {
		t.Fatal("blatt09 should not be abstract")
	}

	// A declared abstract base.
	viper.Set("mpd.defaults", true)
	viper.Set("mpd.defaults.abstract", true)
	viper.Set("mpd.defaults.per", "student")
	if !assignmentIsAbstract("mpd", "defaults") {
		t.Fatal("defaults should be abstract")
	}

	// A child extending an abstract base must NOT itself become abstract:
	// the flag is read from the own value before inheritance is resolved.
	viper.Set("mpd.blatt10", true)
	viper.Set("mpd.blatt10.extends", "defaults")
	viper.Set("mpd.blatt10.assignmentpath", "blatt-10")
	if assignmentIsAbstract("mpd", "blatt10") {
		t.Fatal("blatt10 extends an abstract base but must not be abstract itself")
	}

	// And after full resolution the abstract flag must not leak into the
	// effective config.
	cfg := mustAssignmentConfig(t, "mpd", "blatt10")
	if cfg.Per != PerStudent {
		t.Fatalf("Per = %q, want inherited student", cfg.Per)
	}
	if viper.GetBool("mpd.blatt10.abstract") {
		t.Fatal("abstract flag leaked into resolved blatt10 config")
	}
}

func TestDeepMerge(t *testing.T) {
	parent := map[string]any{
		"a": 1,
		"b": map[string]any{"x": 1, "y": 2},
		"c": []string{"keep"},
	}
	child := map[string]any{
		"b": map[string]any{"y": 20, "z": 30},
		"c": []string{"replaced"},
		"d": 4,
	}

	got := deepMerge(parent, child)

	if got["a"] != 1 {
		t.Fatalf("a = %v, want inherited 1", got["a"])
	}
	if got["d"] != 4 {
		t.Fatalf("d = %v, want 4", got["d"])
	}
	bm, ok := asStringMap(got["b"])
	if !ok {
		t.Fatalf("b is not a map: %#v", got["b"])
	}
	if bm["x"] != 1 || bm["y"] != 20 || bm["z"] != 30 {
		t.Fatalf("merged b = %#v, want {x:1 y:20 z:30}", bm)
	}
	cs, ok := got["c"].([]string)
	if !ok || len(cs) != 1 || cs[0] != "replaced" {
		t.Fatalf("c = %#v, want slice replaced wholesale", got["c"])
	}
}
