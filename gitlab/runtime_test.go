package gitlab

import (
	"net/http"
	"testing"

	"github.com/obcode/glabs/v3/config"
	"github.com/spf13/viper"
)

func TestGenerateReturnsErrorForInvalidPer(t *testing.T) {
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

	if err := client.Generate(cfg, false); err == nil {
		t.Fatal("Generate with an invalid per succeeded, want an error")
	}
}

func TestProtectToBranchReturnsErrorForInvalidPer(t *testing.T) {
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

	if err := client.ProtectToBranch(cfg); err == nil {
		t.Fatal("ProtectToBranch with an invalid per succeeded, want an error")
	}
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
