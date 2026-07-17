package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/obcode/glabs/v3/web/graph/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// EnsureUserIndexes makes the email unique. Called once at startup.
func (db *DB) EnsureUserIndexes(ctx context.Context) error {
	_, err := db.collection(collectionUsers).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("cannot create users index: %w", err)
	}
	return nil
}

// GetUserByEmail returns the user with the given email, or nil if there is none.
// The auth middleware treats nil as "not on the allowlist" — a 403 — so a missing
// user must be nil, nil rather than an error.
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	var user model.User
	err := db.collection(collectionUsers).FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read user %s: %w", email, err)
	}
	return &user, nil
}

// SaveUser inserts or updates a user, keyed by email.
func (db *DB) SaveUser(ctx context.Context, user *model.User) error {
	_, err := db.collection(collectionUsers).ReplaceOne(ctx,
		bson.M{"email": user.Email}, user,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("cannot save user %s: %w", user.Email, err)
	}
	return nil
}

// CountUsers reports how many users exist, so seeding can run only on an empty
// allowlist.
func (db *DB) CountUsers(ctx context.Context) (int64, error) {
	return db.collection(collectionUsers).CountDocuments(ctx, bson.M{})
}
