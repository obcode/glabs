// Package db is glabs-web's MongoDB layer. It owns the connection and the
// collection access; everything above it works with the decoded documents.
package db

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Collection names. One place, so a typo is a compile error rather than a silent
// empty result.
const (
	collectionUsers       = "users"
	collectionCourses     = "courses"
	collectionUserSecrets = "user_secrets"
	collectionActivity    = "activity"
	collectionJobs        = "scheduled_jobs"
)

type DB struct {
	client   *mongo.Client
	database string
}

// Connect opens the connection and verifies it with a ping, so a bad URI fails
// at startup rather than on the first query.
//
// UseLocalTimeZone decodes the UTC that Mongo stores back into time.Local, which
// main sets to Europe/Berlin — so timestamps read out in the zone they were
// written in.
func Connect(ctx context.Context, uri, database string) (*DB, error) {
	if uri == "" {
		return nil, fmt.Errorf("db.uri is required")
	}
	if database == "" {
		return nil, fmt.Errorf("db.database is required")
	}

	client, err := mongo.Connect(
		options.Client().
			ApplyURI(uri).
			SetBSONOptions(&options.BSONOptions{UseLocalTimeZone: true}),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to mongodb: %w", err)
	}
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("cannot reach mongodb at the configured uri: %w", err)
	}

	return &DB{client: client, database: database}, nil
}

func (db *DB) Disconnect(ctx context.Context) error {
	return db.client.Disconnect(ctx)
}

func (db *DB) collection(name string) *mongo.Collection {
	return db.client.Database(db.database).Collection(name)
}
