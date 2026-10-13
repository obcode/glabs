package db

import (
	"errors"
	"time"
)

// Job status values. A job starts pending, is claimed to running, and ends in
// exactly one terminal state.
const (
	JobPending   = "pending"
	JobRunning   = "running"
	JobDone      = "done"
	JobFailed    = "failed"
	JobExpired   = "expired"
	JobCancelled = "cancelled"
)

// jobRetention keeps finished jobs around for 30 days. PG.ReapExpired deletes on
// the runner's tick, where `finished_at < cutoff` excludes the unfinished ones by
// itself — so pending and running jobs are never removed. One number, here, next
// to the reason for it.
const jobRetention = 30 * 24 * time.Hour

// ErrNoDueJob is returned by ClaimDueJob when there is nothing to run.
var ErrNoDueJob = errors.New("no due job")

// ErrJobNotFound is returned when a job does not exist for the given owner (or is
// no longer in a state that permits the requested change).
var ErrJobNotFound = errors.New("scheduled job not found")

// ScheduledJob is one mutating operation queued to run at a wall-clock time. It
// is the persistent unit the poll-runner claims and executes; because it is
// stored rather than held in memory, jobs survive restarts, missed runs are
// caught up after downtime, and an atomic claim keeps two runners from firing the
// same job. Ownership is strict, exactly like courses.
type ScheduledJob struct {
	ID         string
	Owner      string
	Op         string
	Course     string
	Assignment string
	OnlyFor    []string
	Params     map[string]string
	RunAt      time.Time
	// ConfigHash is copied from the confirm token; the runner re-resolves and
	// compares it at fire time, refusing a job whose config drifted since planning.
	ConfigHash string
	Status     string
	GraceMin   int
	CreatedAt  time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
	Log        string
	Err        string
	Notified   bool
	WorkerID   string
}
