package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/obcode/glabs/v3/web/db/sqlc"
)

// newJobID replaces bson.NewObjectID().Hex(). Ids stay TEXT and stay generated
// in Go: production is full of 24-hex ObjectId strings that the GUI passes back
// and forth, and SaveJob writes the id onto the caller's struct for the caller
// to read afterwards.
func newJobID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("cannot generate a job id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

func scheduledJobFromRow(row sqlc.ScheduledJob) (*ScheduledJob, error) {
	params, err := unmarshalStringMap(row.Params)
	if err != nil {
		return nil, err
	}
	return &ScheduledJob{
		ID:         row.ID,
		Owner:      row.Owner,
		Op:         row.Op,
		Course:     row.Course,
		Assignment: row.Assignment,
		OnlyFor:    emptyToNil(row.OnlyFor),
		Params:     params,
		RunAt:      row.RunAt,
		ConfigHash: row.ConfigHash,
		Status:     row.Status,
		GraceMin:   row.GraceMinutes,
		CreatedAt:  row.CreatedAt,
		StartedAt:  row.StartedAt,
		FinishedAt: row.FinishedAt,
		Log:        row.Log,
		Err:        row.Err,
		Notified:   row.Notified,
		WorkerID:   row.WorkerID,
	}, nil
}

func scheduledJobsFromRows(rows []sqlc.ScheduledJob) ([]*ScheduledJob, error) {
	out := make([]*ScheduledJob, 0, len(rows))
	for _, row := range rows {
		job, err := scheduledJobFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, nil
}

// SaveJob inserts a new job, assigning it an id if it has none.
func (db *PG) SaveJob(ctx context.Context, job *ScheduledJob) error {
	if job.ID == "" {
		id, err := newJobID()
		if err != nil {
			return err
		}
		job.ID = id
	}

	params, err := marshalStringMap(job.Params)
	if err != nil {
		return err
	}

	err = db.queries.SaveJob(ctx, sqlc.SaveJobParams{
		ID:           job.ID,
		Owner:        job.Owner,
		Op:           job.Op,
		Course:       job.Course,
		Assignment:   job.Assignment,
		OnlyFor:      job.OnlyFor,
		Params:       params,
		RunAt:        job.RunAt,
		ConfigHash:   job.ConfigHash,
		Status:       job.Status,
		GraceMinutes: job.GraceMin,
		CreatedAt:    job.CreatedAt,
		StartedAt:    job.StartedAt,
		FinishedAt:   job.FinishedAt,
		Log:          job.Log,
		Err:          job.Err,
		Notified:     job.Notified,
		WorkerID:     job.WorkerID,
	})
	if err != nil {
		return fmt.Errorf("cannot save scheduled job: %w", err)
	}
	return nil
}

// ClaimDueJob atomically claims the oldest pending job whose time has come,
// flipping it to running so no other runner can take it. It returns ErrNoDueJob
// when nothing is due.
func (db *PG) ClaimDueJob(ctx context.Context, workerID string, now time.Time) (*ScheduledJob, error) {
	row, err := db.queries.ClaimDueJob(ctx, sqlc.ClaimDueJobParams{Now: now, WorkerID: workerID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNoDueJob
	}
	if err != nil {
		return nil, fmt.Errorf("cannot claim scheduled job: %w", err)
	}
	return scheduledJobFromRow(row)
}

// FinishJob records a terminal state (done/failed/expired) with its log and
// error. An id that no longer exists matches nothing and is not an error.
func (db *PG) FinishJob(ctx context.Context, id, status, logText, errText string) error {
	err := db.queries.FinishJob(ctx, sqlc.FinishJobParams{
		ID:         id,
		Status:     status,
		FinishedAt: time.Now(),
		Log:        logText,
		Err:        errText,
	})
	if err != nil {
		return fmt.Errorf("cannot finish scheduled job: %w", err)
	}
	return nil
}

// MarkNotified flags that the terminal-state email for a job has been sent, so a
// restart mid-notification does not send it twice.
func (db *PG) MarkNotified(ctx context.Context, id string) error {
	if err := db.queries.MarkNotified(ctx, id); err != nil {
		return fmt.Errorf("cannot mark scheduled job notified: %w", err)
	}
	return nil
}

// CancelJob cancels one of the owner's pending jobs. A job that is already
// running or finished cannot be cancelled, and another user's job is invisible:
// both cases return ErrJobNotFound.
func (db *PG) CancelJob(ctx context.Context, owner, id string) (*ScheduledJob, error) {
	row, err := db.queries.CancelJob(ctx, sqlc.CancelJobParams{
		ID: id, Owner: owner, FinishedAt: time.Now(),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("cannot cancel scheduled job: %w", err)
	}
	return scheduledJobFromRow(row)
}

// JobsOf returns the owner's jobs, newest scheduled first, optionally filtered
// to the given statuses. Like courses, there is no read without an owner filter.
func (db *PG) JobsOf(ctx context.Context, owner string, statuses []string) ([]*ScheduledJob, error) {
	rows, err := db.queries.JobsOf(ctx, sqlc.JobsOfParams{Owner: owner, Statuses: statuses})
	if err != nil {
		return nil, fmt.Errorf("cannot read scheduled jobs: %w", err)
	}
	return scheduledJobsFromRows(rows)
}

// UnnotifiedTerminalJobs returns finished jobs (done/failed/expired) whose
// notification email has not been sent yet, across all owners.
func (db *PG) UnnotifiedTerminalJobs(ctx context.Context) ([]*ScheduledJob, error) {
	rows, err := db.queries.UnnotifiedTerminalJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot read unnotified jobs: %w", err)
	}
	return scheduledJobsFromRows(rows)
}

// JobOf returns one of the owner's jobs, or ErrJobNotFound.
func (db *PG) JobOf(ctx context.Context, owner, id string) (*ScheduledJob, error) {
	row, err := db.queries.JobOf(ctx, sqlc.JobOfParams{ID: id, Owner: owner})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read scheduled job: %w", err)
	}
	return scheduledJobFromRow(row)
}

// ReapExpired deletes what MongoDB's TTL indexes used to delete: finished jobs
// after 30 days, events after 180.
//
// PostgreSQL has no TTL, and pg_cron would mean an extension with
// shared_preload_libraries and therefore a custom image. A delete on the
// runner's existing tick is less machinery and has three advantages over the TTL
// monitor it replaces: it is testable, it shows up in the log, and the retention
// period sits in Go next to the comment explaining it rather than inside an index
// option.
func (db *PG) ReapExpired(ctx context.Context, now time.Time) (jobs, events int64, err error) {
	jobs, err = db.queries.ReapExpiredJobs(ctx, now.Add(-jobRetention))
	if err != nil {
		return 0, 0, fmt.Errorf("cannot reap finished jobs: %w", err)
	}
	events, err = db.queries.ReapExpiredEvents(ctx, now.Add(-eventRetention))
	if err != nil {
		return jobs, 0, fmt.Errorf("cannot reap old events: %w", err)
	}
	return jobs, events, nil
}
