package app

import (
	"context"
	"testing"
	"time"
)

func newAdminApp(fs *fakeStore, admins ...string) *App {
	set := map[string]bool{}
	for _, a := range admins {
		set[a] = true
	}
	return &App{db: fs, admins: set, loginSeen: map[string]time.Time{}}
}

// The platform-wide reads are the one place that is NOT owner-scoped, so they must
// refuse anyone not on the admins list — and let admins through.
func TestPlatformReadsAreAdminOnly(t *testing.T) {
	fs := newFakeStore()
	a := newAdminApp(fs, "admin@hm.edu")

	since := time.Now().Add(-time.Hour)
	until := time.Now()

	if _, err := a.PlatformEvents(ctxAs("user@hm.edu"), since); err == nil {
		t.Error("PlatformEvents as non-admin succeeded, want an error")
	}
	if _, err := a.PlatformSummary(ctxAs("user@hm.edu"), since, until); err == nil {
		t.Error("PlatformSummary as non-admin succeeded, want an error")
	}
	if _, err := a.PlatformEvents(context.Background(), since); err == nil {
		t.Error("PlatformEvents without a principal succeeded, want an error")
	}

	if _, err := a.PlatformEvents(ctxAs("admin@hm.edu"), since); err != nil {
		t.Errorf("PlatformEvents as admin: %v", err)
	}
	if _, err := a.PlatformSummary(ctxAs("admin@hm.edu"), since, until); err != nil {
		t.Errorf("PlatformSummary as admin: %v", err)
	}
}

func TestIsAdmin(t *testing.T) {
	a := newAdminApp(newFakeStore(), "admin@hm.edu")
	if !a.IsAdmin(ctxAs("admin@hm.edu")) {
		t.Error("admin not recognized")
	}
	if a.IsAdmin(ctxAs("user@hm.edu")) {
		t.Error("non-admin recognized as admin")
	}
	// Case-insensitive on the email.
	if !a.IsAdmin(ctxAs("ADMIN@HM.EDU")) {
		t.Error("admin check should be case-insensitive")
	}
}

// A busy user's repeated requests collapse to one login event (the throttle);
// rejected logins are always recorded.
func TestLoginThrottleAndRejection(t *testing.T) {
	fs := newFakeStore()
	a := newAdminApp(fs)
	ctx := context.Background()

	a.NoteLogin(ctx, "prof@hm.edu", "Prof", "07")
	a.NoteLogin(ctx, "prof@hm.edu", "Prof", "07") // within the window → not recorded again
	if got := countEvents(fs, "login"); got != 1 {
		t.Errorf("login events = %d, want 1 (throttled)", got)
	}

	a.NoteRejectedLogin(ctx, "stranger@hm.edu", "", "nicht auf der Allowlist")
	a.NoteRejectedLogin(ctx, "", "", "kein Header")
	if got := countEvents(fs, "login-rejected"); got != 2 {
		t.Errorf("rejected-login events = %d, want 2 (not throttled)", got)
	}
}

func countEvents(fs *fakeStore, typ string) int {
	n := 0
	for _, e := range fs.events {
		if e.Type == typ {
			n++
		}
	}
	return n
}
