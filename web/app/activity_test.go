package app

import (
	"context"
	"testing"
)

func TestActivityRecordAndRead(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	a := &App{db: fs}
	ctx := ctxAs(owner)

	if err := a.recordOp(context.Background(), owner,
		&opToken{Owner: owner, Op: "setaccess", Course: "tc", Assignment: "blatt1"},
		"done", "3 repositories"); err != nil {
		t.Fatalf("recordOp: %v", err)
	}
	if err := a.recordOp(context.Background(), owner,
		&opToken{Owner: owner, Op: "archive", Course: "tc", Assignment: "blatt2"},
		"failed", "boom"); err != nil {
		t.Fatalf("recordOp: %v", err)
	}

	// Per-assignment: only blatt1's entry.
	got, err := a.AssignmentActivity(ctx, "tc", "blatt1")
	if err != nil {
		t.Fatalf("AssignmentActivity: %v", err)
	}
	if len(got) != 1 || got[0].Op != "setaccess" || got[0].Status != "done" {
		t.Errorf("assignment activity wrong: %+v", got)
	}

	// Course-wide: both entries.
	all, err := a.CourseActivity(ctx, "tc")
	if err != nil {
		t.Fatalf("CourseActivity: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("course activity = %d entries, want 2", len(all))
	}

	// Every store call scoped to the principal's owner.
	for _, seen := range fs.seenOwners {
		if seen != owner {
			t.Errorf("a store call used owner %q, want %q", seen, owner)
		}
	}
}

// Without an authenticated principal, reading the activity log must refuse.
func TestActivityRequiresAuthentication(t *testing.T) {
	a := &App{db: newFakeStore()}
	if _, err := a.AssignmentActivity(context.Background(), "tc", "blatt1"); err == nil {
		t.Error("AssignmentActivity without a principal succeeded, want an error")
	}
	if _, err := a.CourseActivity(context.Background(), "tc"); err == nil {
		t.Error("CourseActivity without a principal succeeded, want an error")
	}
}
