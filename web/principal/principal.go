// Package principal carries the authenticated user through the request context.
//
// It is its own package so both graph (which sets the user in the auth
// middleware) and app (which reads it to scope every query to that user) can
// depend on it without importing each other.
package principal

import (
	"context"

	"github.com/obcode/glabs/v3/web/graph/model"
)

type contextKey string

const userContextKey contextKey = "authUser"

func WithUser(ctx context.Context, user *model.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

// UserFromContext returns the authenticated user, or nil if there is none. A nil
// user is the caller's signal that the request is unauthenticated — every
// data-access path must treat that as "no access", never as "all access".
func UserFromContext(ctx context.Context) *model.User {
	user, _ := ctx.Value(userContextKey).(*model.User)
	return user
}
