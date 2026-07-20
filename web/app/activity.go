package app

import (
	"context"
	"time"

	"github.com/obcode/glabs/v3/web/db"
)

// AssignmentActivity returns the activity log of one of the caller's assignments,
// newest first — what mutating operations have run against it through the web.
func (a *App) AssignmentActivity(ctx context.Context, course, name string) ([]*db.ActivityEntry, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	return a.db.ActivityFor(ctx, o, course, name)
}

// CourseActivity returns the activity log across a whole course of the caller's,
// newest first — the course page groups it by assignment for a per-assignment
// status.
func (a *App) CourseActivity(ctx context.Context, course string) ([]*db.ActivityEntry, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	return a.db.CourseActivityFor(ctx, o, course)
}

// ActivityLog returns the caller's complete activity log across all their courses,
// newest first — the audit-log dump behind the activity page and its JSON download.
func (a *App) ActivityLog(ctx context.Context) ([]*db.ActivityEntry, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	return a.db.AllActivityFor(ctx, o)
}

// recordOp appends one terminal-state entry to the activity log. It is
// best-effort: a logging failure must never fail the operation itself, so the
// caller decides how to surface the returned error (RunOp streams it as a WARN).
func (a *App) recordOp(ctx context.Context, owner string, tok *opToken, status, detail string) error {
	return a.db.RecordActivity(ctx, &db.ActivityEntry{
		Owner:      owner,
		Course:     tok.Course,
		Assignment: tok.Assignment,
		Op:         tok.Op,
		Params:     tok.Params,
		Status:     status,
		Detail:     detail,
		At:         time.Now(),
	})
}
