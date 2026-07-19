package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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

// jobTTLSeconds keeps finished jobs around for 30 days, then lets MongoDB reap
// them. The TTL index acts only on documents whose finishedAt is a date, so
// pending and running jobs (no finishedAt) are never removed.
const jobTTLSeconds = 30 * 24 * 60 * 60

// ErrNoDueJob is returned by ClaimDueJob when there is nothing to run.
var ErrNoDueJob = errors.New("no due job")

// ErrJobNotFound is returned when a job does not exist for the given owner (or is
// no longer in a state that permits the requested change).
var ErrJobNotFound = errors.New("scheduled job not found")

// ScheduledJob is one mutating operation queued to run at a wall-clock time. It
// is the persistent unit the poll-runner claims and executes; because it lives in
// Mongo, jobs survive restarts, missed runs are caught up after downtime, and an
// atomic claim keeps two runners from firing the same job. Ownership is strict,
// exactly like courses.
type ScheduledJob struct {
	ID         string            `bson:"_id"`
	Owner      string            `bson:"owner"`
	Op         string            `bson:"op"`
	Course     string            `bson:"course"`
	Assignment string            `bson:"assignment"`
	OnlyFor    []string          `bson:"onlyFor,omitempty"`
	Params     map[string]string `bson:"params,omitempty"`
	RunAt      time.Time         `bson:"runAt"`
	// ConfigHash is copied from the confirm token; the runner re-resolves and
	// compares it at fire time, refusing a job whose config drifted since planning.
	ConfigHash string     `bson:"configHash"`
	Status     string     `bson:"status"`
	GraceMin   int        `bson:"graceMinutes"`
	CreatedAt  time.Time  `bson:"createdAt"`
	StartedAt  *time.Time `bson:"startedAt,omitempty"`
	FinishedAt *time.Time `bson:"finishedAt,omitempty"`
	Log        string     `bson:"log,omitempty"`
	Err        string     `bson:"err,omitempty"`
	Notified   bool       `bson:"notified"`
	WorkerID   string     `bson:"workerID,omitempty"`
}

// EnsureJobIndexes indexes the collection for the claim ({status, runAt}), the
// owner's GUI list ({owner, runAt desc}), and a 30-day TTL on finished jobs.
func (db *DB) EnsureJobIndexes(ctx context.Context) error {
	ttl := int32(jobTTLSeconds)
	_, err := db.collection(collectionJobs).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "status", Value: 1}, {Key: "runAt", Value: 1}}},
		{Keys: bson.D{{Key: "owner", Value: 1}, {Key: "runAt", Value: -1}}},
		{Keys: bson.D{{Key: "finishedAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(ttl)},
	})
	if err != nil {
		return fmt.Errorf("cannot create scheduled-jobs indexes: %w", err)
	}
	return nil
}

// SaveJob inserts a new job, assigning it an id if it has none.
func (db *DB) SaveJob(ctx context.Context, job *ScheduledJob) error {
	if job.ID == "" {
		job.ID = bson.NewObjectID().Hex()
	}
	if _, err := db.collection(collectionJobs).InsertOne(ctx, job); err != nil {
		return fmt.Errorf("cannot save scheduled job: %w", err)
	}
	return nil
}

// ClaimDueJob atomically claims the oldest pending job whose time has come,
// flipping it to running so no other runner can take it. It returns ErrNoDueJob
// when nothing is due. This single atomic step is what replaces a distributed
// lock: exactly one runner ever owns a given job.
func (db *DB) ClaimDueJob(ctx context.Context, workerID string, now time.Time) (*ScheduledJob, error) {
	var job ScheduledJob
	err := db.collection(collectionJobs).FindOneAndUpdate(ctx,
		bson.M{"status": JobPending, "runAt": bson.M{"$lte": now}},
		bson.M{"$set": bson.M{"status": JobRunning, "startedAt": now, "workerID": workerID}},
		options.FindOneAndUpdate().
			SetSort(bson.D{{Key: "runAt", Value: 1}}).
			SetReturnDocument(options.After),
	).Decode(&job)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNoDueJob
	}
	if err != nil {
		return nil, fmt.Errorf("cannot claim scheduled job: %w", err)
	}
	return &job, nil
}

// FinishJob records a terminal state (done/failed/expired) with its log and error.
func (db *DB) FinishJob(ctx context.Context, id, status, logText, errText string) error {
	now := time.Now()
	_, err := db.collection(collectionJobs).UpdateByID(ctx, id, bson.M{"$set": bson.M{
		"status": status, "finishedAt": now, "log": logText, "err": errText,
	}})
	if err != nil {
		return fmt.Errorf("cannot finish scheduled job: %w", err)
	}
	return nil
}

// MarkNotified flags that the terminal-state email for a job has been sent, so a
// restart mid-notification does not send it twice.
func (db *DB) MarkNotified(ctx context.Context, id string) error {
	_, err := db.collection(collectionJobs).UpdateByID(ctx, id, bson.M{"$set": bson.M{"notified": true}})
	if err != nil {
		return fmt.Errorf("cannot mark scheduled job notified: %w", err)
	}
	return nil
}

// CancelJob cancels one of the owner's pending jobs. A job that is already running
// or finished cannot be cancelled, and another user's job is invisible: both cases
// return ErrJobNotFound.
func (db *DB) CancelJob(ctx context.Context, owner, id string) (*ScheduledJob, error) {
	now := time.Now()
	var job ScheduledJob
	err := db.collection(collectionJobs).FindOneAndUpdate(ctx,
		bson.M{"_id": id, "owner": owner, "status": JobPending},
		bson.M{"$set": bson.M{"status": JobCancelled, "finishedAt": now}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&job)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("cannot cancel scheduled job: %w", err)
	}
	return &job, nil
}

// JobsOf returns the owner's jobs, newest scheduled first, optionally filtered to
// the given statuses. Like courses, there is no read without an owner filter.
func (db *DB) JobsOf(ctx context.Context, owner string, statuses []string) ([]*ScheduledJob, error) {
	filter := bson.M{"owner": owner}
	if len(statuses) > 0 {
		filter["status"] = bson.M{"$in": statuses}
	}
	cur, err := db.collection(collectionJobs).Find(ctx, filter,
		options.Find().SetSort(bson.D{{Key: "runAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("cannot read scheduled jobs: %w", err)
	}
	var out []*ScheduledJob
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("cannot decode scheduled jobs: %w", err)
	}
	return out, nil
}

// UnnotifiedTerminalJobs returns finished jobs (done/failed/expired) whose
// notification email has not been sent yet, across all owners — the runner's
// notify sweep. Cancelled jobs are excluded (there is no cancellation mail). This
// is what makes "email on every terminal state" survive a crash between finishing
// a job and mailing it: on restart the job is terminal and still unnotified, so
// the mail is sent (once — MarkNotified then guards against a resend).
func (db *DB) UnnotifiedTerminalJobs(ctx context.Context) ([]*ScheduledJob, error) {
	cur, err := db.collection(collectionJobs).Find(ctx, bson.M{
		"notified": false,
		"status":   bson.M{"$in": []string{JobDone, JobFailed, JobExpired}},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot read unnotified jobs: %w", err)
	}
	var out []*ScheduledJob
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("cannot decode unnotified jobs: %w", err)
	}
	return out, nil
}

// JobOf returns one of the owner's jobs, or ErrJobNotFound.
func (db *DB) JobOf(ctx context.Context, owner, id string) (*ScheduledJob, error) {
	var job ScheduledJob
	err := db.collection(collectionJobs).FindOne(ctx, bson.M{"_id": id, "owner": owner}).Decode(&job)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrJobNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read scheduled job: %w", err)
	}
	return &job, nil
}
