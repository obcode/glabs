package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// SystemState returns the single state row, or a zero-valued one (never nil) if
// it somehow does not exist.
//
// The migration seeds the row, so the not-found branch should be unreachable --
// it stays because the caller's contract is "never nil", and the nightly digest
// asks for this on every tick including the first one after a fresh install.
func (db *PG) SystemState(ctx context.Context) (*SystemState, error) {
	row, err := db.queries.SystemState(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return &SystemState{ID: systemStateID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read system state: %w", err)
	}
	return &SystemState{ID: row.ID, SummarySentAt: row.SummarySentAt}, nil
}

// SetSummarySentAt records when the nightly summary was last sent.
func (db *PG) SetSummarySentAt(ctx context.Context, at time.Time) error {
	if err := db.queries.SetSummarySentAt(ctx, &at); err != nil {
		return fmt.Errorf("cannot record summary-sent time: %w", err)
	}
	return nil
}
