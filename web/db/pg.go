package db

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/obcode/glabs/v3/web/db/sqlc"
)

// PG is glabs-web's PostgreSQL store. It implements the same set of methods as
// *DB, the MongoDB one, and web/db/storetest runs the same suite against both --
// that equivalence is what makes the switch a configuration change rather than a
// leap of faith.
//
// The DTOs (StoredCourse, ScheduledJob, Event, ...) and the sentinel errors are
// shared: they live next to the Mongo store and stay exactly as they are. Only
// the way they are read and written changes.
type PG struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
	// uri is kept for DBHost, which names the server without its credentials.
	uri string
}

// NewPG connects and verifies the connection, so a wrong URI fails at startup
// rather than on the first query -- the same promise db.Connect makes.
//
// The session time zone is pinned to Europe/Berlin, but not because the
// application depends on it: pgx decodes timestamptz into time.Local regardless
// of the session setting, and time.Local is Europe/Berlin because main.go says
// so. Removing this line leaves the store contract suite green -- checked, not
// assumed.
//
// It is here so that a psql on the host, an ad-hoc query during an incident and
// the GUI all print the same wall-clock time. That is worth one line; believing
// it is what makes the timestamps correct would not be.
func NewPG(ctx context.Context, uri string) (*PG, error) {
	if uri == "" {
		return nil, fmt.Errorf("db.uri is required")
	}

	cfg, err := pgxpool.ParseConfig(uri)
	if err != nil {
		return nil, fmt.Errorf("cannot parse postgres uri: %w", err)
	}
	cfg.ConnConfig.RuntimeParams["timezone"] = "Europe/Berlin"
	// Names this process in pg_stat_activity, so a connection holding a lock on
	// the host can be traced back to something.
	if _, ok := cfg.ConnConfig.RuntimeParams["application_name"]; !ok {
		cfg.ConnConfig.RuntimeParams["application_name"] = "glabs-web"
	}
	cfg.MaxConns = 10
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.ConnConfig.ConnectTimeout = 10 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("cannot reach postgres at the configured uri: %w", err)
	}

	return &PG{pool: pool, queries: sqlc.New(pool), uri: uri}, nil
}

// Disconnect closes the pool. Named to match the Mongo store so bootstrap does
// not have to know which one it holds.
func (db *PG) Disconnect(_ context.Context) error {
	db.pool.Close()
	return nil
}

// DBHost is host:port with everything else stripped -- no scheme, no path and in
// particular no credentials, because the result is shown in the admin view.
func (db *PG) DBHost() string {
	u, err := url.Parse(db.uri)
	if err != nil || u.Host == "" {
		// Not a URL: either already bare host:port, or something unparseable.
		// Returning it verbatim would risk printing a password, so only the
		// obviously credential-free form is passed through.
		if strings.ContainsAny(db.uri, "@/") {
			return "the configured server"
		}
		return db.uri
	}
	return u.Host
}

// LooksLikeMongoURI reports whether a connection string is a MongoDB one.
//
// This exists for exactly one moment: the evening glabs-web is switched over,
// when the most likely mistake is a new image starting against an unchanged
// .glabs-web.yaml. Without it the server would come up and behave as though
// every user had no courses. bootstrap refuses to start instead.
func LooksLikeMongoURI(uri string) bool {
	return strings.HasPrefix(uri, "mongodb://") || strings.HasPrefix(uri, "mongodb+srv://")
}
