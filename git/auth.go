package git

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/spf13/viper"
)

// GetAuth returns the credential glabs uses to talk to its own GitLab instance:
// the personal access token, over HTTPS.
//
// glabs used to authenticate git with an SSH key (`sshprivatekey`), separate
// from the token it already needed for the API. That is gone: one credential,
// one transport, the same in the CLI and — once it exists — the web server,
// where a per-user SSH key would be a shared identity with access to every
// user's repositories. The token needs the `write_repository` scope in addition
// to `api`.
//
// GitLab accepts the token as the HTTP password with any non-empty username;
// "oauth2" is the conventional one.
func GetAuth() (transport.AuthMethod, error) {
	token := viper.GetString("gitlab.token")
	if token == "" {
		return nil, fmt.Errorf("gitlab.token is required for git operations (needs the api and write_repository scopes)")
	}
	return &githttp.BasicAuth{Username: "oauth2", Password: token}, nil
}

// AuthForURL returns the credential to use when cloning the given URL.
//
// The token authenticates against one host: the configured GitLab instance.
// Under SSH this was invisible — the operator's key worked against any host, so
// a starter repo or deferred branch could live on github.com or another GitLab.
// Over HTTPS with a PAT that no longer holds, and sending the GitLab token to a
// foreign host would leak it. So the token is attached only for the GitLab host;
// any other host is cloned unauthenticated, which works for public repositories
// and fails with a plain "authentication required" for private ones.
func AuthForURL(rawURL string) (transport.AuthMethod, error) {
	onGitLab, err := isConfiguredGitLabHost(rawURL)
	if err != nil {
		return nil, err
	}
	if !onGitLab {
		return nil, nil
	}
	return GetAuth()
}

func isConfiguredGitLabHost(rawURL string) (bool, error) {
	gitlabHost := viper.GetString("gitlab.host")
	if gitlabHost == "" {
		return false, fmt.Errorf("gitlab.host is not configured")
	}
	gl, err := url.Parse(gitlabHost)
	if err != nil {
		return false, fmt.Errorf("gitlab.host %q is not a valid URL: %w", gitlabHost, err)
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return false, fmt.Errorf("cannot parse URL %q: %w", rawURL, err)
	}
	return strings.EqualFold(gl.Host, target.Host), nil
}
