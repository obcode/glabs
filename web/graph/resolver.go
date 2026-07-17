package graph

import "github.com/obcode/glabs/v3/web/app"

// Resolver is the root resolver. It holds the app core and nothing else;
// resolvers gate on auth and delegate, keeping all logic in app.
type Resolver struct {
	app *app.App
}

func NewResolver(a *app.App) *Resolver {
	return &Resolver{app: a}
}
