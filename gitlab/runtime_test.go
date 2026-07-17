package gitlab

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/obcode/glabs/v3/config"
	"github.com/spf13/viper"
)

type exitTriggered struct {
	code int
}

func (e exitTriggered) Error() string {
	return fmt.Sprintf("exit called with code %d", e.code)
}

func withExitCapture(t *testing.T) func() {
	t.Helper()
	origExit := exitFunc
	exitFunc = func(code int) {
		panic(exitTriggered{code: code})
	}
	return func() {
		exitFunc = origExit
	}
}

func assertExitCode(t *testing.T, expected int, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("expected exit with code %d, got no panic", expected)
		}
		e, ok := r.(exitTriggered)
		if !ok {
			t.Fatalf("expected exitTriggered panic, got %T", r)
		}
		if e.code != expected {
			t.Fatalf("exit code = %d, want %d", e.code, expected)
		}
	}()
	fn()
}

func TestGenerate_UsesExitSeamForInvalidPer(t *testing.T) {
	defer withExitCapture(t)()

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups" {
			_, _ = w.Write([]byte(`[{"id":1,"full_path":"mpd/ss26/blatt-01"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course: "mpd",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
		Per:    config.PerFailed,
	}

	assertExitCode(t, 1, func() {
		client.Generate(cfg)
	})
}

func TestProtectToBranch_UsesExitSeamForInvalidPer(t *testing.T) {
	defer withExitCapture(t)()

	client := newContractClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v4/groups" {
			_, _ = w.Write([]byte(`[{"id":1,"full_path":"mpd/ss26/blatt-01"}]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	cfg := &config.AssignmentConfig{
		Course:   "mpd",
		Path:     "mpd/ss26/blatt-01",
		URL:      "https://gitlab.example.org/mpd/ss26/blatt-01",
		Per:      config.PerFailed,
		Branches: []config.BranchRule{{Name: "main", Protect: true}},
	}

	assertExitCode(t, 1, func() {
		client.ProtectToBranch(cfg)
	})
}

func TestNewClientReturnsErrorOnInvalidBaseURL(t *testing.T) {
	_, err := NewClient(WithHost("://invalid-base-url"), WithToken("token"))
	if err == nil {
		t.Fatal("expected an error for an invalid GitLab base URL")
	}
}

func TestNewClientRequiresHostAndToken(t *testing.T) {
	if _, err := NewClient(WithToken("token")); err == nil {
		t.Error("NewClient without a host succeeded, want an error")
	}
	if _, err := NewClient(WithHost("https://gitlab.example.org")); err == nil {
		t.Error("NewClient without a token succeeded, want an error")
	}
}

func TestNewClientFromViper(t *testing.T) {
	viper.Reset()
	defer viper.Reset()
	viper.Set("gitlab.host", "https://gitlab.example.org")
	viper.Set("gitlab.token", "glpat-secret")

	c, err := NewClientFromViper()
	if err != nil {
		t.Fatalf("NewClientFromViper: %v", err)
	}
	if c == nil || c.Client == nil {
		t.Fatal("NewClientFromViper returned an empty client")
	}
}
