package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/obcode/glabs/v3/web/db/sqlc"
	"github.com/obcode/glabs/v3/web/secrets"
)

// userSecretFromRow maps a row to the DTO.
//
// The four gitlab columns are all-or-nothing, enforced by a CHECK constraint, so
// testing one of them is enough to decide whether there is a token at all.
func userSecretFromRow(row sqlc.UserSecret) *UserSecret {
	s := &UserSecret{Owner: row.Owner}
	if row.GitlabKeyVersion != nil {
		s.GitLab = &secrets.SealedValue{
			KeyVersion: *row.GitlabKeyVersion,
			Nonce:      row.GitlabNonce,
			Ciphertext: row.GitlabCiphertext,
		}
		s.GitLabUpdatedAt = row.GitlabUpdatedAt
	}
	return s
}

// GetUserSecret returns the stored secrets for a user, or nil when none exist.
//
// (nil, nil) rather than a not-found error: every caller reads a nil result as
// "no token configured", which is the normal state on a first visit.
func (db *PG) GetUserSecret(ctx context.Context, owner string) (*UserSecret, error) {
	row, err := db.queries.GetUserSecret(ctx, owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read secrets for %s: %w", owner, err)
	}
	return userSecretFromRow(row), nil
}

// SaveUserGitLabToken upserts the sealed GitLab PAT for a user, touching only
// the gitlab columns so it never clobbers other secrets on the row.
func (db *PG) SaveUserGitLabToken(ctx context.Context, owner string, sealed secrets.SealedValue, updatedAt time.Time) error {
	err := db.queries.SaveUserGitLabToken(ctx, sqlc.SaveUserGitLabTokenParams{
		Owner:            owner,
		GitlabKeyVersion: &sealed.KeyVersion,
		GitlabNonce:      sealed.Nonce,
		GitlabCiphertext: sealed.Ciphertext,
		GitlabUpdatedAt:  &updatedAt,
	})
	if err != nil {
		return fmt.Errorf("cannot save GitLab token for %s: %w", owner, err)
	}
	return nil
}

// DeleteUserGitLabToken removes only the GitLab PAT from a user's secrets; the
// row stays. A user who never had one is not an error.
func (db *PG) DeleteUserGitLabToken(ctx context.Context, owner string) error {
	if err := db.queries.DeleteUserGitLabToken(ctx, owner); err != nil {
		return fmt.Errorf("cannot delete GitLab token for %s: %w", owner, err)
	}
	return nil
}
