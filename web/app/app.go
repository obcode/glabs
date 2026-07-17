// Package app is glabs-web's core: the layer the GraphQL resolvers delegate to,
// holding the database and enforcing that every request acts only as its own
// user. Resolvers stay thin — auth gate plus a call into here — so the rules
// live in one place rather than scattered across the schema.
package app

import (
	"context"
	"strings"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/graph/model"
	"github.com/spf13/viper"
)

type App struct {
	db *db.DB
}

func New(database *db.DB) *App {
	return &App{db: database}
}

// GetUserByEmail looks up a user for the auth middleware's allowlist check.
func (a *App) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return a.db.GetUserByEmail(ctx, email)
}

// LocalDevUser is the identity used when auth is disabled (local development). It
// is never consulted when auth is enabled.
func (a *App) LocalDevUser() *model.User {
	email := strings.ToLower(strings.TrimSpace(viper.GetString("auth.devuser")))
	if email == "" {
		email = "local@localhost"
	}
	return &model.User{Email: email, Name: "Local Dev User"}
}
