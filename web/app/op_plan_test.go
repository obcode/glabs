package app

import (
	"strings"
	"testing"
)

func TestPlanOp_setaccessPlanAndTokenRoundTrip(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}
	ctx := ctxAs(owner)

	plan, err := a.PlanOp(ctx, "setaccess", "uc", "blatt1", nil, nil)
	if err != nil {
		t.Fatalf("PlanOp: %v", err)
	}
	if plan.Op != "setaccess" || plan.Course != "uc" || plan.Assignment != "blatt1" {
		t.Fatalf("plan header wrong: %+v", plan)
	}
	if len(plan.Targets) != 2 {
		t.Fatalf("targets = %d, want 2 (two students)", len(plan.Targets))
	}
	for _, tg := range plan.Targets {
		if tg.For == "" || tg.Repo == "" || !strings.HasPrefix(tg.URL, "https://gl/") {
			t.Errorf("bad target: %+v", tg)
		}
	}
	if plan.Destructive || plan.ConfirmPhrase != "" {
		t.Errorf("setaccess should not be destructive: %+v", plan)
	}
	if plan.Token == "" || plan.ExpiresAt.IsZero() {
		t.Fatal("expected a token with an expiry")
	}

	// The token round-trips to the right payload (also exercises openOpToken).
	tok, err := a.openOpToken(plan.Token)
	if err != nil {
		t.Fatalf("openOpToken: %v", err)
	}
	if tok.Owner != owner || tok.Op != "setaccess" || tok.Course != "uc" || tok.Assignment != "blatt1" {
		t.Errorf("token payload wrong: %+v", tok)
	}
	if tok.ConfigHash == "" {
		t.Error("token is missing the config hash")
	}
}

func TestPlanOp_deleteIsDestructive(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}

	plan, err := a.PlanOp(ctxAs(owner), "delete", "uc", "blatt1", nil, nil)
	if err != nil {
		t.Fatalf("PlanOp: %v", err)
	}
	if !plan.Destructive || plan.ConfirmPhrase != "uc/blatt1" {
		t.Fatalf("delete should be destructive with a confirm phrase: %+v", plan)
	}
}

func TestPlanOp_unknownOpRejected(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}

	if _, err := a.PlanOp(ctxAs(owner), "frobnicate", "uc", "blatt1", nil, nil); err == nil {
		t.Fatal("expected an error for an unknown op")
	}
}

func TestPlanOp_noSealerRejected(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: nil}

	_, err := a.PlanOp(ctxAs(owner), "setaccess", "uc", "blatt1", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "secret storage is unavailable") {
		t.Fatalf("error = %v, want unavailable secret storage", err)
	}
}

func TestPlanOp_missingAssignmentRejected(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}

	if _, err := a.PlanOp(ctxAs(owner), "setaccess", "uc", "nope", nil, nil); err == nil {
		t.Fatal("expected an error for a missing assignment")
	}
}
