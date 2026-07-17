package graph

import (
	"context"
	"net/http"
	"strings"

	"github.com/obcode/glabs/v3/web/graph/model"
	"github.com/obcode/glabs/v3/web/principal"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// UserFromContext returns the authenticated user, or nil.
func UserFromContext(ctx context.Context) *model.User {
	return principal.UserFromContext(ctx)
}

// authProvider is the narrow slice of the app the middleware needs, so it can be
// tested without a database.
type authProvider interface {
	LocalDevUser() *model.User
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
}

// authMiddleware trusts the identity injected by the auth proxy (oauth2-proxy →
// Caddy sets X-Remote-User to the verified OIDC email). It is fail-closed: no
// header is 401, an unknown user is 403 — only allowlisted users get in. With
// auth.enabled=false it injects a local dev user instead, so development needs no
// proxy.
//
// The whole model rests on the server being reachable only through the proxy. If
// it is ever exposed directly, the header is trusted unconditionally and anyone
// can set it — so the deployment must never publish the server's port.
func authMiddleware(p authProvider) func(http.Handler) http.Handler {
	enabled := viper.GetBool("auth.enabled")

	header := strings.TrimSpace(viper.GetString("auth.header"))
	if header == "" {
		header = "X-Remote-User"
	}
	nameHeader := strings.TrimSpace(viper.GetString("auth.displaynameheader"))
	if nameHeader == "" {
		nameHeader = "X-Remote-Displayname"
	}

	if !enabled {
		log.Warn().Msg("auth is DISABLED (auth.enabled=false) — every request runs as the local dev user")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var user *model.User
			if !enabled {
				user = p.LocalDevUser()
			} else {
				email := strings.ToLower(strings.TrimSpace(r.Header.Get(header)))
				if email == "" {
					http.Error(w, "unauthenticated: no identity from the auth proxy", http.StatusUnauthorized)
					return
				}
				u, err := p.GetUserByEmail(r.Context(), email)
				if err != nil {
					log.Error().Err(err).Str("email", email).Msg("cannot verify user")
					http.Error(w, "cannot verify user", http.StatusInternalServerError)
					return
				}
				if u == nil {
					log.Warn().Str("email", email).Msg("rejected login of user not on the allowlist")
					http.Error(w, "forbidden: user not authorized", http.StatusForbidden)
					return
				}
				if u.Name == "" {
					u.Name = strings.TrimSpace(r.Header.Get(nameHeader))
				}
				user = u
			}
			ctx := principal.WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
