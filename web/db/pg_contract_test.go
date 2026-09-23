package db_test

import (
	"testing"

	"github.com/obcode/glabs/v3/internal/pgtest"
	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/db/storetest"
)

// The PostgreSQL store satisfies the contract at compile time; the suite below
// checks that it also satisfies it at run time.
var _ storetest.Store = (*db.PG)(nil)

// TestPGStoreContract runs the store contract against PostgreSQL.
//
// It is the same suite TestMongoStoreContract runs, which is the whole point:
// "PostgreSQL behaves like MongoDB" is a test rather than a hope, and a red test
// here means the new store differs from the one in production.
//
// Skipped unless GLABS_TEST_PG_URI points at a PostgreSQL. In the dev container:
//
//	GLABS_TEST_PG_URI=postgres://glabs@localhost:5432/glabs?sslmode=disable go test ./web/db/...
func TestPGStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Store {
		return pgtest.NewDB(t)
	})
}
