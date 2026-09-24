package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/obcode/glabs/v3/web/db/sqlc"
)

func userAccessFromRow(row sqlc.User) *UserAccess {
	return &UserAccess{
		Email:       row.Email,
		Name:        row.Name,
		Department:  row.Department,
		Status:      row.Status,
		Reason:      row.Reason,
		RequestedAt: row.RequestedAt,
		DecidedAt:   row.DecidedAt,
		DecidedBy:   row.DecidedBy,
	}
}

// GetUserAccess returns the access row for an email, or nil when the person
// never asked.
func (db *PG) GetUserAccess(ctx context.Context, email string) (*UserAccess, error) {
	row, err := db.queries.GetUser(ctx, email)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read access of %s: %w", email, err)
	}
	return userAccessFromRow(row), nil
}

// ListUserAccess returns every row, open requests first.
func (db *PG) ListUserAccess(ctx context.Context) ([]*UserAccess, error) {
	rows, err := db.queries.ListUsers(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot list users: %w", err)
	}
	out := make([]*UserAccess, 0, len(rows))
	for _, row := range rows {
		out = append(out, userAccessFromRow(row))
	}
	return out, nil
}

// InsertAccessRequest stores a pending request. created is false when a row for
// the email already existed; the existing row is then left untouched, which is
// what makes a repeated request harmless.
func (db *PG) InsertAccessRequest(ctx context.Context, u *UserAccess) (created bool, err error) {
	_, err = db.queries.InsertAccessRequest(ctx, sqlc.InsertAccessRequestParams{
		Email:       u.Email,
		Name:        u.Name,
		Department:  u.Department,
		Reason:      u.Reason,
		RequestedAt: u.RequestedAt,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("cannot store access request of %s: %w", u.Email, err)
	}
	return true, nil
}

// SetUserAccessStatus records an admin's decision and returns the updated row,
// or nil when there is no row for the email.
func (db *PG) SetUserAccessStatus(ctx context.Context, email, status, decidedBy string, decidedAt time.Time) (*UserAccess, error) {
	row, err := db.queries.SetUserStatus(ctx, sqlc.SetUserStatusParams{
		Email:     email,
		Status:    status,
		DecidedAt: &decidedAt,
		DecidedBy: decidedBy,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot set access of %s: %w", email, err)
	}
	return userAccessFromRow(row), nil
}

// DeleteUserAccess removes the row, so the person is back to "never asked". It
// reports whether there was a row.
func (db *PG) DeleteUserAccess(ctx context.Context, email string) (bool, error) {
	n, err := db.queries.DeleteUser(ctx, email)
	if err != nil {
		return false, fmt.Errorf("cannot delete access of %s: %w", email, err)
	}
	return n > 0, nil
}
