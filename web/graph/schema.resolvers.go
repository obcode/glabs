package graph

import (
	"context"
	"fmt"

	"github.com/obcode/glabs/v3/web/graph/generated"
	"github.com/obcode/glabs/v3/web/graph/model"
)

// Me is the resolver for the me field.
func (r *queryResolver) Me(ctx context.Context) (*model.User, error) {
	if user := UserFromContext(ctx); user != nil {
		return user, nil
	}
	return nil, fmt.Errorf("no authenticated user in context")
}

// ServerInfo is the resolver for the serverInfo field.
func (r *queryResolver) ServerInfo(ctx context.Context) (*model.ServerInfo, error) {
	return r.app.ServerInfo(), nil
}

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

type queryResolver struct{ *Resolver }
