package app

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/obcode/glabs/v3/web/secrets"
)

func testSealer(t *testing.T) *secrets.Sealer {
	t.Helper()
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	s, err := secrets.NewSealer(key)
	if err != nil {
		t.Fatalf("NewSealer: %v", err)
	}
	return s
}

// A stored token round-trips as status only: set reports set=true, the status
// query reports set=true without ever handing back the token, and remove clears
// it — all scoped to the authenticated principal.
func TestGitLabTokenLifecycle(t *testing.T) {
	fs := newFakeStore()
	a := &App{db: fs, sealer: testSealer(t)}
	ctx := ctxAs("prof@hm.edu")

	if st, err := a.SetGitLabToken(ctx, "glpat-secret"); err != nil {
		t.Fatalf("set: %v", err)
	} else if !st.Set || st.UpdatedAt == nil {
		t.Fatalf("set status = %+v, want set=true with timestamp", st)
	}

	// The token is sealed at rest, never stored in plaintext.
	sec := fs.userSecret["prof@hm.edu"]
	if sec == nil || sec.GitLab == nil {
		t.Fatal("token was not stored")
	}
	if plain, _ := a.sealer.Open(*sec.GitLab); plain != "glpat-secret" {
		t.Fatalf("sealed token did not round-trip: %q", plain)
	}

	if st, err := a.GitLabTokenStatus(ctx); err != nil {
		t.Fatalf("status: %v", err)
	} else if !st.Set {
		t.Fatal("status reports no token after set")
	}

	if st, err := a.RemoveGitLabToken(ctx); err != nil {
		t.Fatalf("remove: %v", err)
	} else if st.Set {
		t.Fatal("status still set after remove")
	}
	if st, _ := a.GitLabTokenStatus(ctx); st.Set {
		t.Fatal("status still set after remove")
	}
}

// Without a KEK the app must fail closed rather than persist a plaintext token.
func TestSetGitLabTokenFailsClosedWithoutSealer(t *testing.T) {
	fs := newFakeStore()
	a := &App{db: fs, sealer: nil}
	if _, err := a.SetGitLabToken(ctxAs("prof@hm.edu"), "glpat-secret"); err == nil {
		t.Fatal("expected error when no sealer is configured")
	}
	if len(fs.userSecret) != 0 {
		t.Fatal("token stored despite missing sealer")
	}
}

// Every token operation must require an authenticated principal.
func TestGitLabTokenRequiresPrincipal(t *testing.T) {
	a := &App{db: newFakeStore(), sealer: testSealer(t)}
	ctx := context.Background()
	if _, err := a.SetGitLabToken(ctx, "x"); err == nil {
		t.Fatal("set without principal should fail")
	}
	if _, err := a.GitLabTokenStatus(ctx); err == nil {
		t.Fatal("status without principal should fail")
	}
	if _, err := a.RemoveGitLabToken(ctx); err == nil {
		t.Fatal("remove without principal should fail")
	}
}
