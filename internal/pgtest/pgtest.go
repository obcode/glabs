// Package pgtest gives each test its own PostgreSQL database.
//
// Without GLABS_TEST_PG_URI the tests skip, unless GLABS_TEST_PG_REQUIRED is
// set, which turns the skip into a failure. CI sets both, so a database that
// fails to come up breaks the build instead of quietly skipping every test --
// without that, "green" and "never ran" are indistinguishable.
//
// The databases are created from a TEMPLATE rather than migrated one by one.
// Today that saves little; it is the shape that keeps the suite fast as the
// schema grows, and it is what plexams.go arrived at with 64 tables.
//
// Deliberately no Testcontainers. The dev container mounts no Docker socket, so
// a container per test could not start there at all; pointing an environment
// variable at a database that already exists works in both places.
package pgtest

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/obcode/glabs/v3/web/db"
)

const (
	uriEnv      = "GLABS_TEST_PG_URI"
	requiredEnv = "GLABS_TEST_PG_REQUIRED"

	templatePrefix = "glabs_tmpl_"
	testPrefix     = "glabs_test_"
)

var (
	templateOnce sync.Once
	templateName string
	templateErr  error
)

// NewDB returns a *db.PG on a fresh, migrated database, dropped when the test
// ends.
func NewDB(t *testing.T) *db.PG {
	t.Helper()

	uri := os.Getenv(uriEnv)
	if uri == "" {
		if os.Getenv(requiredEnv) != "" {
			t.Fatalf("%s is set but %s is empty", requiredEnv, uriEnv)
		}
		t.Skipf("set %s to run the PostgreSQL store tests", uriEnv)
	}

	templateOnce.Do(func() { templateName, templateErr = ensureTemplate(uri) })
	if templateErr != nil {
		t.Fatalf("cannot prepare the template database: %v", templateErr)
	}

	name := fmt.Sprintf("%s%d_%s", testPrefix, time.Now().UnixNano(), sanitize(t.Name()))
	ctx := t.Context()

	if err := withMaintenance(ctx, uri, func(conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, fmt.Sprintf("create database %s template %s",
			quote(name), quote(templateName)))
		return err
	}); err != nil {
		t.Fatalf("cannot create test database: %v", err)
	}

	t.Cleanup(func() {
		// A fresh context: the test's own is already cancelled by now.
		dropCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := withMaintenance(dropCtx, uri, func(conn *pgx.Conn) error {
			_, err := conn.Exec(dropCtx, "drop database if exists "+quote(name)+" with (force)")
			return err
		}); err != nil {
			t.Logf("cannot drop test database %s: %v", name, err)
		}
	})

	pg, err := db.NewPG(ctx, replaceDatabase(uri, name))
	if err != nil {
		t.Fatalf("cannot connect to test database: %v", err)
	}
	t.Cleanup(func() { _ = pg.Disconnect(context.Background()) })

	return pg
}

// ensureTemplate creates and migrates the template database once per schema. The
// name carries the checksum of the migrations, so editing one produces a new
// template instead of silently reusing a stale schema -- the failure mode that
// would otherwise cost an afternoon.
func ensureTemplate(uri string) (string, error) {
	checksum, err := db.MigrationsChecksum()
	if err != nil {
		return "", err
	}
	name := templatePrefix + checksum

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Several `go test` packages may run at once. The advisory lock makes the
	// create-and-migrate sequence atomic across processes; the existence check
	// inside it means only the first one does the work.
	err = withMaintenance(ctx, uri, func(conn *pgx.Conn) error {
		if _, err := conn.Exec(ctx, "select pg_advisory_lock($1)", lockKey(name)); err != nil {
			return fmt.Errorf("cannot take the advisory lock: %w", err)
		}
		defer conn.Exec(ctx, "select pg_advisory_unlock($1)", lockKey(name)) //nolint:errcheck

		var exists bool
		if err := conn.QueryRow(ctx,
			"select exists (select 1 from pg_database where datname = $1)", name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return nil
		}

		if _, err := conn.Exec(ctx, "create database "+quote(name)); err != nil {
			return err
		}

		// Migrate through a pool that is closed again immediately: CREATE
		// DATABASE ... TEMPLATE refuses to run while anyone is connected to the
		// template.
		pool, err := pgxpool.New(ctx, replaceDatabase(uri, name))
		if err != nil {
			return err
		}
		migrateErr := db.MigratePG(ctx, pool)
		pool.Close()
		if migrateErr != nil {
			// Leave nothing half-migrated behind to be reused as a template.
			_, _ = conn.Exec(ctx, "drop database if exists "+quote(name)+" with (force)")
			return migrateErr
		}

		dropStaleTemplates(ctx, conn, name)
		return nil
	})
	if err != nil {
		return "", err
	}

	return name, nil
}

// dropStaleTemplates removes templates of earlier schema versions. Without this
// a developer's server slowly fills up with one database per migration they ever
// edited.
func dropStaleTemplates(ctx context.Context, conn *pgx.Conn, keep string) {
	rows, err := conn.Query(ctx,
		"select datname from pg_database where datname like $1 and datname <> $2",
		templatePrefix+"%", keep)
	if err != nil {
		return
	}
	var stale []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err == nil {
			stale = append(stale, name)
		}
	}
	rows.Close()

	for _, name := range stale {
		// Best effort: another process may still be using it.
		_, _ = conn.Exec(ctx, "drop database if exists "+quote(name))
	}
}

// withMaintenance runs fn on a single connection to the server named in uri. A
// plain connection rather than a pool, because CREATE/DROP DATABASE cannot run
// in a transaction and must not be spread over changing sessions -- and because
// an advisory lock is held by a session, not by a pool.
func withMaintenance(ctx context.Context, uri string, fn func(*pgx.Conn) error) error {
	conn, err := pgx.Connect(ctx, uri)
	if err != nil {
		return fmt.Errorf("cannot connect to %s: %w", redact(uri), err)
	}
	defer conn.Close(ctx) //nolint:errcheck

	return fn(conn)
}

func replaceDatabase(uri, name string) string {
	cfg, err := pgx.ParseConfig(uri)
	if err != nil {
		return uri
	}
	auth := cfg.User
	if cfg.Password != "" {
		auth += ":" + cfg.Password
	}
	return fmt.Sprintf("postgres://%s@%s:%d/%s?sslmode=disable", auth, cfg.Host, cfg.Port, name)
}

func redact(uri string) string {
	cfg, err := pgx.ParseConfig(uri)
	if err != nil {
		return "the configured server"
	}
	return fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
}

// quote makes an identifier safe to interpolate. Database names cannot be bound
// as parameters, so this is the only defence -- and the names here are derived
// from test names, which are attacker-free but not syntax-free.
func quote(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

// maxTestNameLen keeps the whole database name inside PostgreSQL's 63-byte
// identifier limit: len("glabs_test_") + 19 digits of UnixNano + "_" = 31.
const maxTestNameLen = 63 - len(testPrefix) - 19 - 1

// sanitize turns a test name into something that survives being a database name.
func sanitize(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if len(s) > maxTestNameLen {
		s = s[:maxTestNameLen]
	}
	return s
}

func lockKey(name string) int64 {
	h := fnv.New64a()
	h.Write([]byte(name))
	return int64(h.Sum64()) //nolint:gosec // wraparound is fine for a lock key
}
