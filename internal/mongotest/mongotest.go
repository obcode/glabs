// Package mongotest gives each test its own MongoDB database.
//
// Without GLABS_TEST_MONGO_URI the tests skip, unless GLABS_TEST_MONGO_REQUIRED
// is set, which turns the skip into a failure. CI sets both, so a database that
// fails to come up breaks the build instead of quietly skipping every test —
// without that, "green" and "never ran" are indistinguishable.
//
// There is deliberately no Testcontainers here. The dev container shares its
// network namespace with a MongoDB and mounts no Docker socket, so a container
// per test could not start there at all; pointing an environment variable at a
// database that already exists works in both places, and is faster in CI.
//
// This package exists only for the move to PostgreSQL: it is what lets the
// store contract suite run against the implementation being replaced. It goes
// away with the MongoDB store.
package mongotest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/obcode/glabs/v3/web/db"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	uriEnv      = "GLABS_TEST_MONGO_URI"
	requiredEnv = "GLABS_TEST_MONGO_REQUIRED"

	testPrefix = "glabs_test_"
)

// NewDB returns a *db.DB on a fresh database, dropped when the test ends.
func NewDB(t *testing.T) *db.DB {
	t.Helper()
	database, _, _ := NewDBWithURI(t)
	return database
}

// NewDBWithURI is NewDB plus the connection string and database name it used.
//
// The import tool takes those two as configuration rather than as a handle --
// it is a command, not a library -- so a test that drives it end to end needs
// them.
//
// It creates the indexes the server creates at startup, because some of them
// are not decoration: the unique (owner, name) on courses is what makes a
// second save of the same course a replace rather than a second document.
func NewDBWithURI(t *testing.T) (*db.DB, string, string) {
	t.Helper()

	uri := os.Getenv(uriEnv)
	if uri == "" {
		if os.Getenv(requiredEnv) != "" {
			t.Fatalf("%s is set but %s is empty", requiredEnv, uriEnv)
		}
		t.Skipf("set %s to run the MongoDB store tests", uriEnv)
	}

	name := fmt.Sprintf("%s%d_%s", testPrefix, time.Now().UnixNano(), sanitize(t.Name()))
	ctx := t.Context()

	database, err := db.Connect(ctx, uri, name)
	if err != nil {
		t.Fatalf("cannot connect to the test database: %v", err)
	}
	t.Cleanup(func() {
		// A fresh context: the test's own is already cancelled by now.
		dropCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := drop(dropCtx, uri, name); err != nil {
			t.Logf("cannot drop test database %s: %v", name, err)
		}
		if err := database.Disconnect(dropCtx); err != nil {
			t.Logf("cannot disconnect from %s: %v", name, err)
		}
	})

	for _, ensure := range []struct {
		what string
		fn   func(context.Context) error
	}{
		{"courses", database.EnsureCourseIndexes},
		{"user_secrets", database.EnsureUserSecretIndexes},
		{"activity", database.EnsureActivityIndexes},
		{"scheduled_jobs", database.EnsureJobIndexes},
		{"events", database.EnsureEventIndexes},
	} {
		if err := ensure.fn(ctx); err != nil {
			t.Fatalf("cannot create the %s indexes: %v", ensure.what, err)
		}
	}

	return database, uri, name
}

// drop removes the test database through a connection of its own. db.DB does not
// expose its client, and it should not: dropping a whole database is a thing
// tests do, not a thing the application may do.
func drop(ctx context.Context, uri, name string) error {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return fmt.Errorf("cannot connect: %w", err)
	}
	defer client.Disconnect(ctx) //nolint:errcheck
	return client.Database(name).Drop(ctx)
}

// maxTestNameLen keeps the whole database name inside MongoDB's 63-byte limit:
// len("glabs_test_") + 19 digits of UnixNano + "_" = 31.
const maxTestNameLen = 63 - len(testPrefix) - 19 - 1

// sanitize turns a test name into something that survives being a database name.
// MongoDB rejects /\. "$*<>:|? outright, and a subtest name is full of slashes.
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
