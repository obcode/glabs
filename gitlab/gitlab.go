package gitlab

import (
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

type Client struct {
	*gitlab.Client
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
}

type Option func(*clientOptions)

func WithHost(host string) Option   { return func(o *clientOptions) { o.host = host } }
func WithToken(token string) Option { return func(o *clientOptions) { o.token = token } }

// WithHTTPClient injects the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(o *clientOptions) { o.httpClient = hc }
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

	return &Client{client}, nil
}

// NewClientFromViper builds a client from the global config. It is the CLI's
// entry point, keeping the single-token model; the web server uses NewClient
// with a per-user token instead.
func NewClientFromViper() (*Client, error) {
	return NewClient(
		WithHost(viper.GetString("gitlab.host")),
		WithToken(viper.GetString("gitlab.token")),
	)
}
