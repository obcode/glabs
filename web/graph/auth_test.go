package graph

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obcode/glabs/v3/web/graph/model"
	"github.com/spf13/viper"
)

// fakeAuthProvider stands in for the app, so the middleware is tested without a
// database.
type fakeAuthProvider struct {
	dev        *model.User
	logins     int
	rejections int
	lastDept   string
	lastName   string
}

func (f *fakeAuthProvider) LocalDevUser() *model.User { return f.dev }
func (f *fakeAuthProvider) NoteLogin(_ context.Context, _, name, department string) {
	f.logins++
	f.lastName = name
	f.lastDept = department
}
func (f *fakeAuthProvider) NoteRejectedLogin(_ context.Context, _, department, _ string) {
	f.rejections++
	f.lastDept = department
}

// capture records the user the middleware placed in the context, so a test can
// assert both the HTTP status and who the request ended up running as.
func capture(seen **model.User) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

func serve(t *testing.T, p authProvider, header, value string) (int, *model.User) {
	t.Helper()
	var seen *model.User
	h := authMiddleware(p)(capture(&seen))

	req := httptest.NewRequest(http.MethodPost, "/query", nil)
	if header != "" {
		req.Header.Set(header, value)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, seen
}

func TestAuthMiddlewareRejectsMissingHeader(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("auth.enabled", true)

	code, user := serve(t, &fakeAuthProvider{}, "", "")
	if code != http.StatusUnauthorized {
		t.Errorf("no header: status = %d, want 401", code)
	}
	if user != nil {
		t.Errorf("no header: a user reached the handler: %+v", user)
	}
}

// There is no allowlist: any identity the proxy asserts is let in and acts as its
// own user. The email is lowercased so a differently-cased header still yields a
// canonical identity.
func TestAuthMiddlewareAcceptsAnyAuthenticatedUser(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("auth.enabled", true)

	code, user := serve(t, &fakeAuthProvider{}, "X-Remote-User", "STRANGER@HM.EDU")
	if code != http.StatusOK {
		t.Fatalf("any user: status = %d, want 200", code)
	}
	if user == nil || user.Email != "stranger@hm.edu" {
		t.Errorf("any user: context user = %+v, want stranger@hm.edu", user)
	}
}

// An accepted login is noted once, carrying the display name and department
// headers. This is what feeds the monitoring log.
func TestAuthMiddlewareNotesLoginWithNameAndDepartment(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("auth.enabled", true)

	p := &fakeAuthProvider{}
	h := authMiddleware(p)(capture(new(*model.User)))
	req := httptest.NewRequest(http.MethodPost, "/query", nil)
	req.Header.Set("X-Remote-User", "prof@hm.edu")
	req.Header.Set("X-Remote-Displayname", "Prof Example")
	req.Header.Set("X-Remote-Department", "07")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if p.logins != 1 || p.rejections != 0 {
		t.Errorf("accepted login: logins=%d rejections=%d, want 1/0", p.logins, p.rejections)
	}
	if p.lastName != "Prof Example" {
		t.Errorf("name = %q, want %q", p.lastName, "Prof Example")
	}
	if p.lastDept != "07" {
		t.Errorf("department = %q, want %q", p.lastDept, "07")
	}
}

// A missing identity header is the one rejection left, and it is noted as such.
func TestAuthMiddlewareNotesRejectionOnMissingHeader(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("auth.enabled", true)

	p := &fakeAuthProvider{}
	code, _ := serve(t, p, "", "")
	if code != http.StatusUnauthorized || p.rejections != 1 || p.logins != 0 {
		t.Errorf("missing header: code=%d logins=%d rejections=%d, want 401/0/1", code, p.logins, p.rejections)
	}
}

// With auth disabled the middleware never consults the header — every request
// runs as the local dev user. This is the local-development path; it must not
// accidentally depend on a header being present.
func TestAuthMiddlewareDisabledUsesDevUser(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("auth.enabled", false)

	dev := &model.User{Email: "dev@localhost", Name: "Dev"}
	p := &fakeAuthProvider{dev: dev}

	code, user := serve(t, p, "", "")
	if code != http.StatusOK {
		t.Fatalf("disabled: status = %d, want 200", code)
	}
	if user == nil || user.Email != "dev@localhost" {
		t.Errorf("disabled: context user = %+v, want the dev user", user)
	}
}
