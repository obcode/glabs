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
	allow      map[string]*model.User
	dev        *model.User
	err        error
	logins     int
	rejections int
	lastDept   string
}

func (f *fakeAuthProvider) LocalDevUser() *model.User { return f.dev }
func (f *fakeAuthProvider) GetUserByEmail(_ context.Context, email string) (*model.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.allow[email], nil
}
func (f *fakeAuthProvider) NoteLogin(_ context.Context, _, _, department string) {
	f.logins++
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

func TestAuthMiddlewareRejectsUnknownUser(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("auth.enabled", true)

	code, user := serve(t, &fakeAuthProvider{allow: map[string]*model.User{}}, "X-Remote-User", "stranger@hm.edu")
	if code != http.StatusForbidden {
		t.Errorf("unknown user: status = %d, want 403", code)
	}
	if user != nil {
		t.Errorf("unknown user: a user reached the handler: %+v", user)
	}
}

func TestAuthMiddlewareAcceptsAllowlistedUser(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("auth.enabled", true)

	prof := &model.User{Email: "prof@hm.edu", Name: "Prof"}
	p := &fakeAuthProvider{allow: map[string]*model.User{"prof@hm.edu": prof}}

	// The email is lowercased before lookup, so a differently-cased header still
	// matches.
	code, user := serve(t, p, "X-Remote-User", "PROF@HM.EDU")
	if code != http.StatusOK {
		t.Fatalf("allowlisted user: status = %d, want 200", code)
	}
	if user == nil || user.Email != "prof@hm.edu" {
		t.Errorf("allowlisted user: context user = %+v, want prof@hm.edu", user)
	}
}

func TestAuthMiddlewareInternalErrorOnLookupFailure(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("auth.enabled", true)

	p := &fakeAuthProvider{err: context.DeadlineExceeded}
	code, _ := serve(t, p, "X-Remote-User", "prof@hm.edu")
	if code != http.StatusInternalServerError {
		t.Errorf("lookup failure: status = %d, want 500", code)
	}
}

// An accepted login is noted once, carrying the department header; a rejected one
// is noted as a rejection. This is what feeds the monitoring log.
func TestAuthMiddlewareNotesLoginWithDepartment(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("auth.enabled", true)

	prof := &model.User{Email: "prof@hm.edu", Name: "Prof"}
	p := &fakeAuthProvider{allow: map[string]*model.User{"prof@hm.edu": prof}}

	h := authMiddleware(p)(capture(new(*model.User)))
	req := httptest.NewRequest(http.MethodPost, "/query", nil)
	req.Header.Set("X-Remote-User", "prof@hm.edu")
	req.Header.Set("X-Remote-Department", "07")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if p.logins != 1 || p.rejections != 0 {
		t.Errorf("accepted login: logins=%d rejections=%d, want 1/0", p.logins, p.rejections)
	}
	if p.lastDept != "07" {
		t.Errorf("department = %q, want %q", p.lastDept, "07")
	}

	// A stranger is a rejection, not a login.
	p2 := &fakeAuthProvider{allow: map[string]*model.User{}}
	code, _ := serve(t, p2, "X-Remote-User", "stranger@hm.edu")
	if code != http.StatusForbidden || p2.rejections != 1 || p2.logins != 0 {
		t.Errorf("stranger: code=%d logins=%d rejections=%d, want 403/0/1", code, p2.logins, p2.rejections)
	}
}

// With auth disabled the middleware never consults the header or the database —
// every request runs as the local dev user. This is the local-development path;
// it must not accidentally depend on a header being present.
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
