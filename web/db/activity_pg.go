package db

import (
	"context"
	"fmt"

	"github.com/obcode/glabs/v3/web/db/sqlc"
)

// activityCap is activityLimit as the nullable limit the queries take. A nil
// limit means no limit -- `limit null` in PostgreSQL -- which is how the
// uncapped audit dump is expressed.
func activityCap() *int32 {
	n := int32(activityLimit)
	return &n
}

func activityEntryFromRow(row sqlc.Activity) (*ActivityEntry, error) {
	params, err := unmarshalStringMap(row.Params)
	if err != nil {
		return nil, err
	}
	return &ActivityEntry{
		Owner:      row.Owner,
		Course:     row.Course,
		Assignment: row.Assignment,
		Op:         row.Op,
		Params:     params,
		Status:     row.Status,
		Detail:     row.Detail,
		At:         row.At,
	}, nil
}

func activityEntriesFromRows(rows []sqlc.Activity) ([]*ActivityEntry, error) {
	out := make([]*ActivityEntry, 0, len(rows))
	for _, row := range rows {
		e, err := activityEntryFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// RecordActivity appends one entry to the log.
func (db *PG) RecordActivity(ctx context.Context, e *ActivityEntry) error {
	params, err := marshalStringMap(e.Params)
	if err != nil {
		return err
	}
	err = db.queries.RecordActivity(ctx, sqlc.RecordActivityParams{
		Owner:      e.Owner,
		Course:     e.Course,
		Assignment: e.Assignment,
		Op:         e.Op,
		Params:     params,
		Status:     e.Status,
		Detail:     e.Detail,
		At:         e.At,
	})
	if err != nil {
		return fmt.Errorf("cannot record activity: %w", err)
	}
	return nil
}

// ActivityFor returns the log entries of one assignment, newest first, capped.
func (db *PG) ActivityFor(ctx context.Context, owner, course, assignment string) ([]*ActivityEntry, error) {
	rows, err := db.queries.ActivityFor(ctx, sqlc.ActivityForParams{
		Owner: owner, Course: course, Assignment: assignment, Lim: activityCap(),
	})
	if err != nil {
		return nil, fmt.Errorf("cannot read activity: %w", err)
	}
	return activityEntriesFromRows(rows)
}

// CourseActivityFor returns the log entries across a whole course, newest first,
// capped.
func (db *PG) CourseActivityFor(ctx context.Context, owner, course string) ([]*ActivityEntry, error) {
	rows, err := db.queries.CourseActivityFor(ctx, sqlc.CourseActivityForParams{
		Owner: owner, Course: course, Lim: activityCap(),
	})
	if err != nil {
		return nil, fmt.Errorf("cannot read activity: %w", err)
	}
	return activityEntriesFromRows(rows)
}

// AllActivityFor returns the owner's complete log, newest first and UNCAPPED:
// a dump has to be complete, and one user's trail is bounded in practice.
func (db *PG) AllActivityFor(ctx context.Context, owner string) ([]*ActivityEntry, error) {
	rows, err := db.queries.AllActivityFor(ctx, sqlc.AllActivityForParams{Owner: owner, Lim: nil})
	if err != nil {
		return nil, fmt.Errorf("cannot read activity: %w", err)
	}
	return activityEntriesFromRows(rows)
}
