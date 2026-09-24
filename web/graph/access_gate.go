package graph

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/obcode/glabs/v3/web/app"
)

// approvalChecker is the slice of the app the access gate needs.
type approvalChecker interface {
	IsApproved(ctx context.Context) bool
}

// openRootFields are the only operations a user who is authenticated but not
// approved may run: enough to find out where they stand and to ask to be let in.
// Introspection (__schema, __type) is on the list because it reveals nothing the
// public repository does not; in production it is switched off anyway.
var openRootFields = map[string]bool{
	"me":            true,
	"serverInfo":    true,
	"requestAccess": true,
	"__schema":      true,
	"__type":        true,
	"__typename":    true,
}

var rootObjects = map[string]bool{"Query": true, "Mutation": true, "Subscription": true}

// accessGate refuses every root field outside openRootFields to a user who is not
// approved. It is a list of what is open, not of what is closed, so a field added
// later is closed until someone decides otherwise.
//
// It is a field middleware rather than a check in the HTTP middleware because
// the request itself has to get through: the GUI reads `me` to learn that it
// should show the request-access page. Nested fields pass without a check; they
// can only be reached through a root field that already did.
func accessGate(p approvalChecker) graphql.FieldMiddleware {
	return func(ctx context.Context, next graphql.Resolver) (any, error) {
		fc := graphql.GetFieldContext(ctx)
		if fc == nil || !rootObjects[fc.Object] || openRootFields[fc.Field.Name] {
			return next(ctx)
		}
		if !p.IsApproved(ctx) {
			return nil, app.ErrNotApproved
		}
		return next(ctx)
	}
}
