package config

import "testing"

// inheritanceCourse is a parent assignment (blatt09) with every kind of nested
// structure, plus the children that inherit from it. The children override only
// what differs — which is the point of the whole feature and the reason the
// merge has to run on the raw map: `pipeline` is never mentioned again below,
// and it has to survive.
const inheritanceCourse = `
mpd:
  coursepath: mpd
  semesterpath: ss26
  useCoursenameAsPrefix: true
  students:
    - alice

  blatt09:
    assignmentpath: blatt-09
    description: Blatt 9
    per: student
    mergeRequest:
      mergeMethod: semi_linear
      squashOption: never
      pipeline: true
    startercode:
      url: git@gitlab.lrz.de:mpd/labs/blatt-09.git
      fromBranch: startercode
      template: true
    branches:
      - name: main
        mergeOnly: true
    deferredBranches:
      devcontainer:
        url: git@gitlab.lrz.de:mpd/devcontainer.git
        fromBranch: main
        toBranch: devcontainer
      solution:
        fromBranch: solution
        orphan: true
        orphanMessage: Lösung 9

  # Overrides one nested startercode field; everything else is inherited.
  blatt10:
    extends: blatt09
    assignmentpath: blatt-10
    description: Blatt 10
    startercode:
      url: git@gitlab.lrz.de:mpd/labs/blatt-10.git

  # Overrides one field of one deferred branch.
  blatt10deep:
    extends: blatt09
    assignmentpath: blatt-10-deep
    deferredBranches:
      solution:
        orphanMessage: Lösung 10

  # Third level: inherits through blatt10.
  blatt11:
    extends: blatt10
    assignmentpath: blatt-11
    description: Blatt 11

  # No extends at all.
  standalone:
    assignmentpath: standalone
    description: Standalone
`

// A child inherits everything it does not mention, and a nested map is merged
// field by field rather than replaced wholesale.
func TestInheritanceOverridesAndInherits(t *testing.T) {
	registerCourse(t, inheritanceCourse)
	cfg := mustAssignmentConfig(t, "mpd", "blatt10")

	// Overridden.
	if cfg.Path != "mpd/ss26/blatt-10" {
		t.Errorf("Path = %q, want %q", cfg.Path, "mpd/ss26/blatt-10")
	}
	if cfg.Description != "Blatt 10" {
		t.Errorf("Description = %q, want %q", cfg.Description, "Blatt 10")
	}
	if cfg.Startercode.URL != "git@gitlab.lrz.de:mpd/labs/blatt-10.git" {
		t.Errorf("Startercode.URL = %q, want the override", cfg.Startercode.URL)
	}

	// Inherited, including the sibling fields of the overridden one: the
	// startercode block was merged, not replaced.
	if cfg.Startercode.FromBranch != "startercode" {
		t.Errorf("Startercode.FromBranch = %q, want %q inherited", cfg.Startercode.FromBranch, "startercode")
	}
	if !cfg.Startercode.Template {
		t.Error("Startercode.Template = false, want true inherited")
	}
	if cfg.MergeRequest.MergeMethod != SemiLinearHistory {
		t.Errorf("MergeMethod = %q, want %q inherited", cfg.MergeRequest.MergeMethod, SemiLinearHistory)
	}

	// The one that a struct-level merge would have got wrong: the child never
	// mentions pipeline, so its zero value must not overwrite the parent's true.
	if !cfg.MergeRequest.PipelineMustSucceed {
		t.Error("PipelineMustSucceed = false: an unmentioned field overwrote the inherited value")
	}

	if len(cfg.Branches) != 1 || !cfg.Branches[0].MergeOnly {
		t.Errorf("Branches = %#v, want the inherited main rule", cfg.Branches)
	}
	if len(cfg.DeferredBranches) != 2 {
		t.Errorf("DeferredBranches = %v, want both inherited", cfg.DeferredBranches)
	}
}

// Deep merge reaches into a map inside a map: overriding one field of one
// deferred branch must keep the branch's other fields and the sibling branch.
func TestInheritanceDeepMergesNestedDeferredBranch(t *testing.T) {
	registerCourse(t, inheritanceCourse)
	cfg := mustAssignmentConfig(t, "mpd", "blatt10deep")

	solution := cfg.DeferredBranches["solution"]
	if solution == nil {
		t.Fatalf("solution branch missing; got %v", cfg.DeferredBranches)
	}
	if solution.OrphanMessage != "Lösung 10" {
		t.Errorf("OrphanMessage = %q, want the override", solution.OrphanMessage)
	}
	if solution.FromBranch != "solution" {
		t.Errorf("FromBranch = %q, want %q inherited", solution.FromBranch, "solution")
	}
	if !solution.Orphan {
		t.Error("Orphan = false, want true inherited")
	}
	if cfg.DeferredBranches["devcontainer"] == nil {
		t.Error("devcontainer branch lost: the deferredBranches map was replaced instead of merged")
	}
}

func TestInheritanceMultiLevelChain(t *testing.T) {
	registerCourse(t, inheritanceCourse)
	cfg := mustAssignmentConfig(t, "mpd", "blatt11")

	if cfg.Description != "Blatt 11" {
		t.Errorf("Description = %q, want %q", cfg.Description, "Blatt 11")
	}
	// From blatt10, one level up.
	if cfg.Startercode.URL != "git@gitlab.lrz.de:mpd/labs/blatt-10.git" {
		t.Errorf("Startercode.URL = %q, want the value from blatt10", cfg.Startercode.URL)
	}
	// From blatt09, two levels up.
	if cfg.Startercode.FromBranch != "startercode" {
		t.Errorf("Startercode.FromBranch = %q, want %q from blatt09", cfg.Startercode.FromBranch, "startercode")
	}
	if !cfg.MergeRequest.PipelineMustSucceed {
		t.Error("PipelineMustSucceed = false, want true from blatt09")
	}
}

func TestInheritanceLeavesUnrelatedAssignmentsAlone(t *testing.T) {
	registerCourse(t, inheritanceCourse)
	cfg := mustAssignmentConfig(t, "mpd", "standalone")

	if cfg.Startercode != nil {
		t.Errorf("Startercode = %#v, want nil: standalone extends nothing", cfg.Startercode)
	}
	if cfg.MergeRequest.PipelineMustSucceed {
		t.Error("PipelineMustSucceed = true: config leaked from another assignment")
	}
}

func TestAbstractFlagIsReadBeforeInheritance(t *testing.T) {
	registerCourse(t, `
mpd:
  coursepath: mpd
  defaults:
    abstract: true
    per: student
  blatt10:
    extends: defaults
    assignmentpath: blatt-10
`)

	// The base itself cannot be operated on.
	if _, err := GetAssignmentConfig("mpd", "defaults"); err == nil {
		t.Fatal("an abstract assignment was accepted, want an error")
	}

	// A child extending it must not become abstract: the flag is read from the
	// assignment's own value, before inheritance is resolved.
	cfg, err := GetAssignmentConfig("mpd", "blatt10")
	if err != nil {
		t.Fatalf("blatt10 extends an abstract base but must not be abstract itself: %v", err)
	}
	if cfg.Per != PerStudent {
		t.Fatalf("Per = %q, want %q inherited from the base", cfg.Per, PerStudent)
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
