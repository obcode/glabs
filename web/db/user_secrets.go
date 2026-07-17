package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/obcode/glabs/v3/web/secrets"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// UserSecret holds a user's encrypted per-user secrets, keyed by the owner's email
// — here, the GitLab personal access token. The value is AES-256-GCM sealed; the
// plaintext never touches the database. This document is never exposed over
// GraphQL, only a "set / when" status is.
type UserSecret struct {
	Owner           string               `bson:"owner"`
	GitLab          *secrets.SealedValue `bson:"gitlab,omitempty"`
	GitLabUpdatedAt *time.Time           `bson:"gitlabUpdatedAt,omitempty"`
}

// EnsureUserSecretIndexes makes owner unique — one secrets document per user.
func (db *DB) EnsureUserSecretIndexes(ctx context.Context) error {
	_, err := db.collection(collectionUserSecrets).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "owner", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("cannot create user_secrets index: %w", err)
	}
	return nil
}

// GetUserSecret returns the stored secrets for a user, or nil when none exist.
func (db *DB) GetUserSecret(ctx context.Context, owner string) (*UserSecret, error) {
	var s UserSecret
	err := db.collection(collectionUserSecrets).FindOne(ctx, bson.M{"owner": owner}).Decode(&s)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read secrets for %s: %w", owner, err)
	}
	return &s, nil
}

// SaveUserGitLabToken upserts the sealed GitLab PAT for a user, touching only the
// gitlab fields so it never clobbers other secrets on the document.
func (db *DB) SaveUserGitLabToken(ctx context.Context, owner string, sealed secrets.SealedValue, updatedAt time.Time) error {
	_, err := db.collection(collectionUserSecrets).UpdateOne(ctx,
		bson.M{"owner": owner},
		bson.M{"$set": bson.M{"owner": owner, "gitlab": sealed, "gitlabUpdatedAt": updatedAt}},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return fmt.Errorf("cannot save GitLab token for %s: %w", owner, err)
	}
	return nil
}

// DeleteUserGitLabToken removes only the GitLab PAT from a user's secrets.
func (db *DB) DeleteUserGitLabToken(ctx context.Context, owner string) error {
	_, err := db.collection(collectionUserSecrets).UpdateOne(ctx,
		bson.M{"owner": owner},
		bson.M{"$unset": bson.M{"gitlab": "", "gitlabUpdatedAt": ""}},
	)
	if err != nil {
		return fmt.Errorf("cannot delete GitLab token for %s: %w", owner, err)
	}
	return nil
}
