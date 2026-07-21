package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Event types. Unlike the owner-scoped activity log (which records only the six
// mutating GitLab ops of a single user), the event log is the platform-wide
// monitoring trail an operator reads: who signed in, which jobs were scheduled and
// how they ended, and anything else worth watching. It is deliberately NOT
// owner-scoped — an admin reads across all users.
const (
	EventLogin         = "login"          // a user was active (throttled: at most once per window)
	EventLoginRejected = "login-rejected" // an unauthenticated/not-allowlisted request was refused
	EventJobScheduled  = "job-scheduled"  // an operation was queued to run later
	EventJobDone       = "job-done"
	EventJobFailed     = "job-failed"
	EventJobExpired    = "job-expired"
	EventJobCancelled  = "job-cancelled"
	EventOpDone        = "op-done" // an interactive (run-now) operation finished
	EventOpFailed      = "op-failed"
	EventCourseCreated = "course-created"
	EventCourseDeleted = "course-deleted"
	EventTokenSaved    = "token-saved"
	EventTokenDeleted  = "token-deleted"
)

// Event severities, used to highlight what matters in the digest and the admin
// page. They live here (not in config) to stay independent of the linter's
// Severity type.
const (
	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityError   = "error"
)

// eventTTLSeconds keeps events for 180 days, then lets MongoDB reap them. The
// monitoring trail is a rolling window, not a permanent archive.
const eventTTLSeconds = 180 * 24 * 60 * 60

// Event is one thing that happened on the platform, worth an operator's
// attention. Most fields are optional and depend on the type: a login carries an
// actor (and maybe a department) but no course; a job event carries course,
// assignment and op. Ownership is NOT enforced — this collection exists precisely
// to be read across all users by an admin.
type Event struct {
	At   time.Time `bson:"at"`
	Type string    `bson:"type"`
	// Actor is the acting user's email (lowercased); empty for an anonymous
	// rejected login (no identity header at all).
	Actor      string `bson:"actor,omitempty"`
	ActorName  string `bson:"actorName,omitempty"`
	Department string `bson:"department,omitempty"`
	Course     string `bson:"course,omitempty"`
	Assignment string `bson:"assignment,omitempty"`
	Op         string `bson:"op,omitempty"`
	Severity   string `bson:"severity"`
	Detail     string `bson:"detail,omitempty"`
	JobID      string `bson:"jobID,omitempty"`
}

// EnsureEventIndexes indexes the log for the newest-first reads (the digest window
// and the admin page) and a filter-by-type, plus a 180-day TTL.
func (db *DB) EnsureEventIndexes(ctx context.Context) error {
	ttl := int32(eventTTLSeconds)
	_, err := db.collection(collectionEvents).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "at", Value: -1}}},
		{Keys: bson.D{{Key: "type", Value: 1}, {Key: "at", Value: -1}}},
		{Keys: bson.D{{Key: "at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(ttl)},
	})
	if err != nil {
		return fmt.Errorf("cannot create event indexes: %w", err)
	}
	return nil
}

// RecordEvent appends one event to the log. Callers set At and Severity; a missing
// severity defaults to info so a forgotten field never hides an event.
func (db *DB) RecordEvent(ctx context.Context, e *Event) error {
	if e.Severity == "" {
		e.Severity = SeverityInfo
	}
	if _, err := db.collection(collectionEvents).InsertOne(ctx, e); err != nil {
		return fmt.Errorf("cannot record event: %w", err)
	}
	return nil
}

// EventsBetween returns every event in [since, until), oldest first — the window
// the nightly digest aggregates. It is cross-user by design.
func (db *DB) EventsBetween(ctx context.Context, since, until time.Time) ([]*Event, error) {
	filter := bson.M{"at": bson.M{"$gte": since, "$lt": until}}
	return db.findEvents(ctx, filter, bson.D{{Key: "at", Value: 1}}, 0)
}

// RecentEvents returns events at or after since, newest first, capped at limit —
// the admin page's live feed. Cross-user by design.
func (db *DB) RecentEvents(ctx context.Context, since time.Time, limit int64) ([]*Event, error) {
	filter := bson.M{"at": bson.M{"$gte": since}}
	return db.findEvents(ctx, filter, bson.D{{Key: "at", Value: -1}}, limit)
}

func (db *DB) findEvents(ctx context.Context, filter bson.M, sort bson.D, limit int64) ([]*Event, error) {
	opts := options.Find().SetSort(sort)
	if limit > 0 {
		opts.SetLimit(limit)
	}
	cur, err := db.collection(collectionEvents).Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("cannot read events: %w", err)
	}
	var out []*Event
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("cannot decode events: %w", err)
	}
	return out, nil
}
