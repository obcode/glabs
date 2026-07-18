package app

import (
	"strings"
	"testing"
)

func TestAssignmentReport_noTokenStoredReturnsError(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gitlab.example", sealer: testSealer(t)}

	// The assignment resolves, but no GitLab token is stored → a clear error.
	_, err := a.AssignmentReport(ctxAs(owner), "uc", "blatt1")
	if err == nil {
		t.Fatal("expected an error when no token is stored")
	}
	if !strings.Contains(err.Error(), "no GitLab token") {
		t.Errorf("error = %q, want it to mention the missing token", err)
	}
}

func TestAssignmentReport_noSealerReturnsError(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gitlab.example", sealer: nil}

	_, err := a.AssignmentReport(ctxAs(owner), "uc", "blatt1")
	if err == nil {
		t.Fatal("expected an error when secret storage is unavailable")
	}
	if !strings.Contains(err.Error(), "secret storage is unavailable") {
		t.Errorf("error = %q, want it to mention unavailable secret storage", err)
	}
}

func TestAssignmentReport_missingAssignmentReturnsNil(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gitlab.example", sealer: testSealer(t)}

	// No such assignment → nil, no error (and no token needed).
	rep, err := a.AssignmentReport(ctxAs(owner), "uc", "nope")
	if err != nil {
		t.Fatalf("AssignmentReport: %v", err)
	}
	if rep != nil {
		t.Errorf("expected nil for a missing assignment, got %+v", rep)
	}
}

func TestAssignmentReport_abstractBaseReturnsNil(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}

	// An abstract base does not resolve → nil, no error (token never consulted).
	rep, err := a.AssignmentReport(ctxAs(owner), "tc", "base")
	if err != nil {
		t.Fatalf("AssignmentReport: %v", err)
	}
	if rep != nil {
		t.Errorf("expected nil for an abstract base, got %+v", rep)
	}
}
