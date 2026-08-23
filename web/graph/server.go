package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	coderws "github.com/coder/websocket"
	"github.com/go-chi/chi/v5"
	"github.com/obcode/glabs/v3/web/app"
	"github.com/obcode/glabs/v3/web/graph/generated"
	"github.com/obcode/glabs/v3/web/graph/model"
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var defaultAllowedOrigins = []string{
	"http://localhost:5173",
	"http://localhost:8080",
	"http://localhost:3000",
}

// serverInfoProvider is the sliver of app.App that the liveness probe needs. An interface,
// not the struct, so the routing can be tested without a MongoDB behind it — and the routing
// is what needs testing here: whether /healthz really sits OUTSIDE the auth group.
type serverInfoProvider interface {
	ServerInfo() *model.ServerInfo
}

// newRouter wires the routes. Separate from StartServer purely so it can be exercised in a
// test: chi panics when Use() is called after a route is registered, and a panic here would
// be an outage at startup rather than a red test.
func newRouter(auth authProvider, info serverInfoProvider, srv http.Handler, production bool, origins []string) chi.Router {
	router := chi.NewRouter()
	router.Use(cors.New(cors.Options{
		AllowedOrigins:   origins,
		AllowCredentials: true,
		AllowedHeaders:   []string{"*"},
	}).Handler)

	// Liveness probe, deliberately OUTSIDE the auth group below.
	//
	// Two reasons it cannot sit behind authMiddleware: a monitor has no OIDC session, and
	// every identity-less request there is recorded as a REJECTED LOGIN — a check running
	// every minute would fill the admin monitoring log with its own noise.
	//
	// It reports the running version, which is the half that matters. That a container
	// started is not the same statement as the image you meant to deploy answering.
	router.Get("/healthz", healthz(info))

	// Everything else is auth-gated. A Group gets its own middleware stack, which is how chi
	// allows one unauthenticated route beside the authenticated ones.
	router.Group(func(r chi.Router) {
		r.Use(authMiddleware(auth))

		if !production {
			r.Handle("/", playground.Handler("glabs-web GraphQL playground", "/query"))
		}
		r.Handle("/query", srv)
	})

	return router
}

// healthz answers the liveness probe on /healthz. Unauthenticated by design — see the route.
//
// Deliberately thin: it does NOT touch MongoDB. This answers "is this process alive and which
// build is it", and it has to keep answering during a database outage, because "the database
// is gone" is a thing the monitor must be able to REPORT rather than time out on. That outage
// reaches us the other way, through the error reporting: a failing query logs at Error level
// and lands in GlitchTip, which mails. Two channels, each for what it can actually tell.
func healthz(p serverInfoProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		info := p.ServerInfo()
		w.Header().Set("Content-Type", "application/json")
		// A cached liveness answer is a lie waiting to happen.
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"version": info.Version,
			"commit":  info.Commit,
		})
	}
}

func allowedOrigins() []string {
	if o := viper.GetStringSlice("server.allowedorigins"); len(o) > 0 {
		return o
	}
	return defaultAllowedOrigins
}

// originHosts turns the allowed origins (full URLs) into host[:port] patterns for
// coder/websocket's OriginPatterns, which matches against the Origin header's host.
func originHosts(origins []string) []string {
	hosts := make([]string, 0, len(origins))
	for _, o := range origins {
		if u, err := url.Parse(o); err == nil && u.Host != "" {
			hosts = append(hosts, u.Host)
		} else {
			hosts = append(hosts, o)
		}
	}
	return hosts
}

// StartServer wires the GraphQL handler behind CORS and the auth middleware, and
// blocks until SIGTERM/Interrupt.
//
// Middleware order is deliberate: CORS first so preflight OPTIONS short-circuit
// before auth runs; auth second so every GraphQL request carries an identity.
func StartServer(a *app.App, port string) {
	srv := handler.New(generated.NewExecutableSchema(generated.Config{Resolvers: NewResolver(a)}))

	origins := allowedOrigins()

	srv.AddTransport(transport.POST{})

	// WebSocket transport for subscriptions (the report progress stream). The auth
	// middleware runs on the HTTP upgrade request, so the identity it injects is
	// already in the connection context — no separate WS auth is needed. The WS
	// upgrade is not covered by CORS preflight, so origins are checked here via
	// coder/websocket's OriginPatterns (host[:port], scheme stripped).
	srv.AddTransport(transport.Websocket{
		// graphql-ws (the browser client) speaks the modern graphql-transport-ws
		// subprotocol, for which KeepAlivePingInterval does NOT apply — only the
		// legacy apollo subprotocol uses it. Send an unsolicited pong every 10s
		// (PongOnlyInterval, the correct field here) so a long report/check stream
		// (minutes, with gaps between progress) is not dropped as idle and then
		// re-subscribed from scratch by the client's retry.
		PongOnlyInterval: 10 * time.Second,
		Implementation: transport.CoderWebsocketImplementation{
			AcceptOptions: coderws.AcceptOptions{
				OriginPatterns: originHosts(origins),
			},
		},
	})

	production := viper.GetBool("server.production")
	if !production {
		srv.Use(extension.Introspection{})
	}

	router := newRouter(a, a, srv, production, origins)

	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:              fmt.Sprintf(":%s", port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info().Str("port", port).Bool("production", production).Msg("glabs-web listening")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Info().Msg("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("shutdown error")
	}
}
