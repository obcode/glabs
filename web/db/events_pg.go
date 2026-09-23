package db

import (
	"context"
	"fmt"
	"time"

	"github.com/obcode/glabs/v3/web/db/sqlc"
)

func eventFromRow(row sqlc.Event) *Event {
	return &Event{
		At:         row.At,
		Type:       row.Type,
		Actor:      row.Actor,
		ActorName:  row.ActorName,
		Department: row.Department,
		Course:     row.Course,
		Assignment: row.Assignment,
		Op:         row.Op,
		Severity:   row.Severity,
		Detail:     row.Detail,
		JobID:      row.JobID,
	}
}

func eventsFromRows(rows []sqlc.Event) []*Event {
	out := make([]*Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, eventFromRow(row))
	}
	return out
}

// RecordEvent appends one event to the log. A missing severity defaults to info
// so a forgotten field never hides an event -- and, as before, the default is
// written back onto the caller's struct.
func (db *PG) RecordEvent(ctx context.Context, e *Event) error {
	if e.Severity == "" {
		e.Severity = SeverityInfo
	}
	err := db.queries.RecordEvent(ctx, sqlc.RecordEventParams{
		At:         e.At,
		Type:       e.Type,
		Severity:   e.Severity,
		Actor:      e.Actor,
		ActorName:  e.ActorName,
		Department: e.Department,
		Course:     e.Course,
		Assignment: e.Assignment,
		Op:         e.Op,
		Detail:     e.Detail,
		JobID:      e.JobID,
	})
	if err != nil {
		return fmt.Errorf("cannot record event: %w", err)
	}
	return nil
}

// EventsBetween returns every event in [since, until), oldest first -- the window
// the nightly digest aggregates. Cross-user by design.
func (db *PG) EventsBetween(ctx context.Context, since, until time.Time) ([]*Event, error) {
	rows, err := db.queries.EventsBetween(ctx, sqlc.EventsBetweenParams{At: since, At_2: until})
	if err != nil {
		return nil, fmt.Errorf("cannot read events: %w", err)
	}
	return eventsFromRows(rows), nil
}

// RecentEvents returns events at or after since, newest first, capped at limit --
// the admin page's live feed. A limit of zero or less means no cap, matching the
// Mongo store. Cross-user by design.
func (db *PG) RecentEvents(ctx context.Context, since time.Time, limit int64) ([]*Event, error) {
	arg := sqlc.RecentEventsParams{At: since}
	if limit > 0 {
		n := int32(limit) //nolint:gosec // a page size, never near the int32 boundary
		arg.Lim = &n
	}
	rows, err := db.queries.RecentEvents(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("cannot read events: %w", err)
	}
	return eventsFromRows(rows), nil
}
