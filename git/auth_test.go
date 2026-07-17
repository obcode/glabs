package git

import (
	"testing"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/viper"
)

func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
}

func TestGetAuthUsesTheTokenOverHTTPS(t *testing.T) {
	resetViper(t)
	viper.Set("gitlab.token", "glpat-secret")

	auth, err := GetAuth()
	if err != nil {
		t.Fatalf("GetAuth() error = %v", err)
	}

	basic, ok := auth.(*githttp.BasicAuth)
	if !ok {
		t.Fatalf("GetAuth() = %T, want *http.BasicAuth", auth)
	}
	if basic.Password != "glpat-secret" {
		t.Errorf("Password = %q, want the token", basic.Password)
	}
	if basic.Username == "" {
		t.Error("Username is empty; GitLab needs a non-empty username with the token")
	}
}

// Without a token there is nothing to authenticate with. This used to be
// allowed — an empty sshprivatekey fell back to the ssh-agent — but the token is
// now mandatory for any git operation.
func TestGetAuthRequiresAToken(t *testing.T) {
	resetViper(t)

	if _, err := GetAuth(); err == nil {
		t.Fatal("GetAuth() succeeded without a token, want an error")
	}
}

// The token is attached only for the configured GitLab host. Sending it to a
// foreign host (a starter repo on github.com, say) would both fail and leak the
// token, so those are cloned unauthenticated.
func TestAuthForURLIsHostScoped(t *testing.T) {
	resetViper(t)
	viper.Set("gitlab.host", "https://gitlab.lrz.de")
	viper.Set("gitlab.token", "glpat-secret")

	onGitLab, err := AuthForURL("https://gitlab.lrz.de/mpd/startercode/blatt-01.git")
	if err != nil {
		t.Fatalf("AuthForURL(gitlab): %v", err)
	}
	if _, ok := onGitLab.(*githttp.BasicAuth); !ok {
		t.Errorf("AuthForURL(gitlab host) = %T, want *http.BasicAuth with the token", onGitLab)
	}

	foreign, err := AuthForURL("https://github.com/foo/bar.git")
	if err != nil {
		t.Fatalf("AuthForURL(github): %v", err)
	}
	if foreign != nil {
		t.Errorf("AuthForURL(foreign host) = %v, want nil: the GitLab token must not be sent to another host", foreign)
	}
}

// A foreign host is allowed even without a token — a public repo needs none. The
// token requirement only bites for the GitLab host itself.
func TestAuthForURLForeignHostWithoutToken(t *testing.T) {
	resetViper(t)
	viper.Set("gitlab.host", "https://gitlab.lrz.de")

	auth, err := AuthForURL("https://github.com/foo/public.git")
	if err != nil {
		t.Fatalf("AuthForURL(github, no token): %v", err)
	}
	if auth != nil {
		t.Errorf("AuthForURL = %v, want nil", auth)
	}
}
