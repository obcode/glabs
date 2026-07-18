package app

import (
	"strings"
	"testing"
)

const seederCourse = `sc:
  coursepath: sc/sem
  semesterpath: ss26
  blatt1:
    per: student
    accesslevel: developer
    students:
      - a@hm.edu
    seeder:
      cmd: make
      args:
        - build
`

func newRunApp(t *testing.T, owner, name, yaml string) *App {
	t.Helper()
	fs := newFakeStore()
	fs.courses[owner+"/"+name] = storedCourse(t, owner, yaml)
	return &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t), ops: newOpGuard()}
}

func planToken(t *testing.T, a *App, owner, op, course, assignment string) string {
	t.Helper()
	plan, err := a.PlanOp(ctxAs(owner), op, course, assignment, nil, nil)
	if err != nil {
		t.Fatalf("PlanOp: %v", err)
	}
	return plan.Token
}

func TestRunOp_badTokenRejected(t *testing.T) {
	a := newRunApp(t, "prof@hm.edu", "uc", urlsCourse)
	if _, err := a.RunOp(ctxAs("prof@hm.edu"), "not-a-real-token", ""); err == nil {
		t.Fatal("expected an error for a bad token")
	}
}

func TestRunOp_wrongOwnerRejected(t *testing.T) {
	a := newRunApp(t, "prof@hm.edu", "uc", urlsCourse)
	token := planToken(t, a, "prof@hm.edu", "setaccess", "uc", "blatt1")

	_, err := a.RunOp(ctxAs("someone-else@hm.edu"), token, "")
	if err == nil || !strings.Contains(err.Error(), "not created by you") {
		t.Fatalf("error = %v, want a wrong-owner rejection", err)
	}
}

func TestRunOp_configChangedRejected(t *testing.T) {
	const owner = "prof@hm.edu"
	a := newRunApp(t, owner, "uc", urlsCourse)
	token := planToken(t, a, owner, "setaccess", "uc", "blatt1")

	// Change the stored config after planning: the hash no longer matches.
	changed := strings.Replace(urlsCourse, "accesslevel: developer", "accesslevel: maintainer", 1)
	fs := a.db.(*fakeStore)
	fs.courses[owner+"/uc"] = storedCourse(t, owner, changed)

	_, err := a.RunOp(ctxAs(owner), token, "")
	if err == nil || !strings.Contains(err.Error(), "configuration changed") {
		t.Fatalf("error = %v, want a config-changed rejection", err)
	}
}

func TestRunOp_seederRejected(t *testing.T) {
	const owner = "prof@hm.edu"
	a := newRunApp(t, owner, "sc", seederCourse)
	token := planToken(t, a, owner, "setaccess", "sc", "blatt1")

	_, err := a.RunOp(ctxAs(owner), token, "")
	if err == nil || !strings.Contains(err.Error(), "seeder") {
		t.Fatalf("error = %v, want a seeder rejection", err)
	}
}

func TestRunOp_destructiveNeedsConfirmPhrase(t *testing.T) {
	const owner = "prof@hm.edu"
	a := newRunApp(t, owner, "uc", urlsCourse)
	token := planToken(t, a, owner, "delete", "uc", "blatt1")

	if _, err := a.RunOp(ctxAs(owner), token, ""); err == nil || !strings.Contains(err.Error(), "confirm") {
		t.Fatalf("empty phrase: error = %v, want a confirm-phrase rejection", err)
	}
	// With the correct phrase the guards pass; then it fails on the missing token
	// (no PAT stored) — synchronously, before streaming.
	if _, err := a.RunOp(ctxAs(owner), token, "uc/blatt1"); err == nil || !strings.Contains(err.Error(), "no GitLab token") {
		t.Fatalf("correct phrase: error = %v, want a missing-token error", err)
	}
}

func TestRunOp_noTokenStoredRejected(t *testing.T) {
	const owner = "prof@hm.edu"
	a := newRunApp(t, owner, "uc", urlsCourse)
	token := planToken(t, a, owner, "setaccess", "uc", "blatt1")

	// All guards pass, but no GitLab token is stored → synchronous error.
	_, err := a.RunOp(ctxAs(owner), token, "")
	if err == nil || !strings.Contains(err.Error(), "no GitLab token") {
		t.Fatalf("error = %v, want a missing-token error", err)
	}
}
