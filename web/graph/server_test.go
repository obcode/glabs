package graph

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/obcode/glabs/v3/web/graph/model"
	"github.com/spf13/viper"
)

// fakeAuth records whether the router let a request reach authMiddleware.
type fakeAuth struct {
	rejected []string
	logins   []string
}

func (f *fakeAuth) LocalDevUser() *model.User { return &model.User{Email: "dev@example.org"} }
func (f *fakeAuth) NoteLogin(_ context.Context, email, _, _ string) {
	f.logins = append(f.logins, email)
}
func (f *fakeAuth) NoteRejectedLogin(_ context.Context, email, _, reason string) {
	f.rejected = append(f.rejected, email+":"+reason)
}

type fakeInfo struct{}

func (fakeInfo) ServerInfo() *model.ServerInfo {
	return &model.ServerInfo{Version: "v9.9.9", Commit: "deadbeef", Date: "2026-08-23"}
}

func testRouter(t *testing.T, auth *fakeAuth) http.Handler {
	t.Helper()
	viper.Set("auth.enabled", true)
	t.Cleanup(func() { viper.Set("auth.enabled", false) })

	backend := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// production=true, so the playground route is not registered — as on the deploy host.
	return newRouter(auth, fakeInfo{}, backend, true, []string{"https://glabs.cs.hm.edu"})
}

// The whole reason /healthz exists as its own route: a monitor has no OIDC session, and it
// must not pay for asking.
func TestHealthzNeedsNoIdentityAndLeavesNoRejectedLogin(t *testing.T) {
	auth := &fakeAuth{}
	rec := httptest.NewRecorder()
	testRouter(t, auth).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}

	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	if got["status"] != "ok" || got["version"] != "v9.9.9" {
		t.Errorf("body = %v", got)
	}

	// This is the assertion that pays for the separate route. Behind authMiddleware every
	// probe would be filed as a refused login, and a check running every minute would bury
	// the real ones in the admin monitoring log.
	if len(auth.rejected) != 0 {
		t.Errorf("the probe was recorded as a rejected login: %v", auth.rejected)
	}
	if len(auth.logins) != 0 {
		t.Errorf("the probe was recorded as a login: %v", auth.logins)
	}
}

// The other half: opening /healthz must not have opened anything else.
func TestQueryStillRequiresIdentity(t *testing.T) {
	auth := &fakeAuth{}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(`{"query":"{__typename}"}`))
	req.Header.Set("Content-Type", "application/json")
	testRouter(t, auth).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 — /query must stay gated", rec.Code)
	}
	if len(auth.rejected) != 1 {
		t.Errorf("a refused request should be recorded once, got %v", auth.rejected)
	}
}

// A cached liveness answer is a lie waiting to happen.
func TestHealthzIsNotCacheable(t *testing.T) {
	rec := httptest.NewRecorder()
	testRouter(t, &fakeAuth{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}
