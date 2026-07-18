package app

import (
	"strings"
	"testing"
)

func TestImportAssignmentYAML_upsertsAndKeepsOthers(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}
	ctx := ctxAs(owner)

	snippet := "blatt2:\n  per: student\n  accesslevel: developer\n  students:\n    - c@hm.edu\n"
	view, err := a.ImportAssignmentYAML(ctx, "uc", snippet)
	if err != nil {
		t.Fatalf("ImportAssignmentYAML: %v", err)
	}
	if view == nil || view.Name != "blatt2" {
		t.Fatalf("view = %+v, want name blatt2", view)
	}

	// The new assignment is stored and resolvable...
	got, err := a.Assignment(ctx, "uc", "blatt2")
	if err != nil {
		t.Fatalf("Assignment(blatt2): %v", err)
	}
	if got == nil || got.ResolveError != "" {
		t.Fatalf("blatt2 not stored/resolvable: %+v", got)
	}
	// ...and the pre-existing assignment is untouched.
	if b1, _ := a.Assignment(ctx, "uc", "blatt1"); b1 == nil {
		t.Error("blatt1 was lost by the import")
	}
}

func TestImportAssignmentYAML_rejectsMultipleTopKeys(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}

	_, err := a.ImportAssignmentYAML(ctxAs(owner), "uc", "a:\n  per: student\nb:\n  per: group\n")
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("error = %v, want a single-top-level-key error", err)
	}
}

func TestImportAssignmentYAML_rejectsBadName(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}

	_, err := a.ImportAssignmentYAML(ctxAs(owner), "uc", "bad name:\n  per: student\n")
	if err == nil || !strings.Contains(err.Error(), "invalid assignment name") {
		t.Fatalf("error = %v, want an invalid-name error", err)
	}
}
