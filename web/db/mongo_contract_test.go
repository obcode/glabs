package db_test

import (
	"testing"

	"github.com/obcode/glabs/v3/internal/mongotest"
	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/db/storetest"
)

// The MongoDB store satisfies the contract at compile time; the suite below
// checks that it also satisfies it at run time.
var _ storetest.Store = (*db.DB)(nil)

// TestMongoStoreContract runs the store contract against MongoDB.
//
// This is the reference run: every assertion in storetest was derived from this
// implementation, which is the one in production. Its job is to be green here
// first, so that when the PostgreSQL store runs the same suite, a red test means
// the new store differs from the old one rather than that the suite is wrong.
//
// Skipped unless GLABS_TEST_MONGO_URI points at a MongoDB. In the dev container:
//
//	GLABS_TEST_MONGO_URI=mongodb://localhost:27017 go test ./web/db/...
func TestMongoStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) storetest.Store {
		return mongotest.NewDB(t)
	})
}
