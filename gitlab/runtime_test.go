package gitlab

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/obcode/glabs/config"
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

func withPanicCapture(t *testing.T, fn func(v interface{})) func() {
	t.Helper()
	origPanic := panicFunc
	panicFunc = fn
	return func() {
		panicFunc = origPanic
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
		Course: "mpd",
		Path:   "mpd/ss26/blatt-01",
		URL:    "https://gitlab.example.org/mpd/ss26/blatt-01",
		Per:    config.PerFailed,
		Startercode: &config.Startercode{
			ToBranch: "main",
		},
	}

	assertExitCode(t, 1, func() {
		client.ProtectToBranch(cfg)
	})
}

func TestNewClient_UsesPanicSeamOnInvalidBaseURL(t *testing.T) {
	triggered := false
	defer withPanicCapture(t, func(v interface{}) {
		triggered = true
		panic(v)
	})()
	viper.Reset()
	defer viper.Reset()
	viper.Set("gitlab.host", "://invalid-base-url")
	viper.Set("gitlab.token", "token")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on invalid GitLab base URL")
		}
		if !triggered {
			t.Fatal("expected panic to pass through panicFunc seam")
		}
	}()

	_ = NewClient()
}
