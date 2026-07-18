package graph

import (
	"context"
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
	"github.com/rs/cors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

var defaultAllowedOrigins = []string{
	"http://localhost:5173",
	"http://localhost:8080",
	"http://localhost:3000",
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
		KeepAlivePingInterval: 10 * time.Second,
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

	router := chi.NewRouter()
	router.Use(cors.New(cors.Options{
		AllowedOrigins:   origins,
		AllowCredentials: true,
		AllowedHeaders:   []string{"*"},
	}).Handler)
	router.Use(authMiddleware(a))

	if !production {
		router.Handle("/", playground.Handler("glabs-web GraphQL playground", "/query"))
	}
	router.Handle("/query", srv)

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
