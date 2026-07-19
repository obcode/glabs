package gitlab

import (
	"fmt"
	"net/http"

	"github.com/obcode/glabs/v3/git"
	"github.com/obcode/glabs/v3/reporter"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

type Client struct {
	*gitlab.Client
	// rep receives progress. It is a field rather than a per-call parameter
	// because the operation methods and their ~20 helpers all report, and
	// because a per-user web request builds its own Client anyway (per-user
	// token), so the reporter is naturally request-scoped and never shared.
	rep reporter.Reporter
	// host and token are kept so the client can build git credentials (git and
	// the API share one token) without reading the global viper config.
	host  string
	token string
	// committer authors starter-code commits; empty falls back to the glabs bot.
	committer git.Committer
}

// clientOptions holds what a Client needs to reach GitLab. They are injected
// rather than read from the global viper config, so the web server can build one
// client per user with that user's token — a package-global token cannot serve
// multiple users. The CLI keeps its single-token behaviour via NewClientFromViper.
type clientOptions struct {
	host           string
	token          string
	httpClient     *http.Client
	withoutRetries bool
	reporter       reporter.Reporter
	committerName  string
	committerEmail string
}

type Option func(*clientOptions)

func WithHost(host string) Option   { return func(o *clientOptions) { o.host = host } }
func WithToken(token string) Option { return func(o *clientOptions) { o.token = token } }

// WithHTTPClient injects the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(o *clientOptions) { o.httpClient = hc }
}

// WithReporter sets where the client's progress goes. Defaults to a
// ConsoleReporter (spinners on stdout); the web server passes a streaming one.
func WithReporter(r reporter.Reporter) Option {
	return func(o *clientOptions) { o.reporter = r }
}

// WithCommitter sets the identity that authors starter-code commits. Empty
// values fall back to the glabs bot identity. The web server passes the acting
// user so a generated commit is attributed to them.
func WithCommitter(name, email string) Option {
	return func(o *clientOptions) {
		o.committerName = name
		o.committerEmail = email
	}
}

// WithoutRetries disables the client's retry-on-5xx wrapper. The contract tests
// use it so a mocked 5xx returns immediately instead of retrying with backoff.
func WithoutRetries() Option {
	return func(o *clientOptions) { o.withoutRetries = true }
}

// NewClient builds a GitLab client from the given options. host and token are
// required; a bad configuration is an error rather than a panic, so a server can
// report it instead of dying.
func NewClient(opts ...Option) (*Client, error) {
	o := &clientOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if o.host == "" {
		return nil, fmt.Errorf("gitlab host is required")
	}
	if o.token == "" {
		return nil, fmt.Errorf("gitlab token is required")
	}

	log.Debug().Str("gitlab.host", o.host).Msg("connecting to gitlab server")

	clientOpts := []gitlab.ClientOptionFunc{gitlab.WithBaseURL(o.host)}
	if o.httpClient != nil {
		clientOpts = append(clientOpts, gitlab.WithHTTPClient(o.httpClient))
	}
	if o.withoutRetries {
		clientOpts = append(clientOpts, gitlab.WithoutRetries())
	}

	client, err := gitlab.NewClient(o.token, clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("cannot create a gitlab client: %w", err)
	}

	rep := o.reporter
	if rep == nil {
		rep = reporter.NewConsoleReporter()
	}

	return &Client{
		Client:    client,
		rep:       rep,
		host:      o.host,
		token:     o.token,
		committer: git.Committer{Name: o.committerName, Email: o.committerEmail},
	}, nil
}

// gitAuth is the credential the client uses for git clone/push: the same token
// as the API, attached only to the configured GitLab host.
func (c *Client) gitAuth() git.TokenAuth {
	return git.TokenAuth{GitLabHost: c.host, Token: c.token}
}

// NewClientFromViper builds a client from the global config. It is the CLI's
// entry point, keeping the single-token model; the web server uses NewClient
// with a per-user token instead.
func NewClientFromViper() (*Client, error) {
	return NewClient(
		WithHost(viper.GetString("gitlab.host")),
		WithToken(viper.GetString("gitlab.token")),
		WithCommitter(viper.GetString("committer.name"), viper.GetString("committer.email")),
	)
}
