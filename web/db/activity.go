package db

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// activityLimit caps how many log entries a single read returns — the GUI shows
// recent history per assignment, not an unbounded audit trail.
const activityLimit = 200

// ActivityEntry records one mutating operation performed through the web against
// an assignment: what ran, when, and how it ended. It is the web's stand-in for
// the shell history the CLI leaves behind — the course page reads it to show,
// per assignment, what has already been done (setaccess, protect, archive,
// delete; later generate). Ownership is strict, exactly like courses: an entry
// belongs to the user who caused it and no other user can see it.
type ActivityEntry struct {
	Owner      string            `bson:"owner"`
	Course     string            `bson:"course"`
	Assignment string            `bson:"assignment"`
	Op         string            `bson:"op"`
	Params     map[string]string `bson:"params,omitempty"`
	// Status is the terminal outcome — "done" or "failed".
	Status string `bson:"status"`
	// Detail is a short human summary: the repository count on success, the error
	// message on failure.
	Detail string    `bson:"detail,omitempty"`
	At     time.Time `bson:"at"`
}

// EnsureActivityIndexes indexes the log for the two reads the GUI makes: the
// newest entries of one assignment, and the newest across a whole course.
func (db *DB) EnsureActivityIndexes(ctx context.Context) error {
	_, err := db.collection(collectionActivity).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "owner", Value: 1}, {Key: "course", Value: 1}, {Key: "assignment", Value: 1}, {Key: "at", Value: -1}}},
		{Keys: bson.D{{Key: "owner", Value: 1}, {Key: "course", Value: 1}, {Key: "at", Value: -1}}},
		// The owner-wide dump (AllActivityFor): every entry of one user, newest first.
		{Keys: bson.D{{Key: "owner", Value: 1}, {Key: "at", Value: -1}}},
	})
	if err != nil {
		return fmt.Errorf("cannot create activity indexes: %w", err)
	}
	return nil
}

// RecordActivity appends one entry to the log. The owner, course and assignment
// on the entry are set by the caller from the authenticated principal.
func (db *DB) RecordActivity(ctx context.Context, e *ActivityEntry) error {
	if _, err := db.collection(collectionActivity).InsertOne(ctx, e); err != nil {
		return fmt.Errorf("cannot record activity: %w", err)
	}
	return nil
}

// ActivityFor returns the log entries of one assignment, newest first.
func (db *DB) ActivityFor(ctx context.Context, owner, course, assignment string) ([]*ActivityEntry, error) {
	return db.findActivity(ctx, bson.M{"owner": owner, "course": course, "assignment": assignment}, activityLimit)
}

// CourseActivityFor returns the log entries across a whole course, newest first —
// the course page groups them by assignment to show each one's latest status.
func (db *DB) CourseActivityFor(ctx context.Context, owner, course string) ([]*ActivityEntry, error) {
	return db.findActivity(ctx, bson.M{"owner": owner, "course": course}, activityLimit)
}

// AllActivityFor returns the owner's complete log across all their courses, newest
// first — the audit-log dump. Unlike the GUI's per-assignment/per-course reads it is
// UNCAPPED (limit 0): a dump must be complete, and one user's audit trail is bounded
// in practice.
func (db *DB) AllActivityFor(ctx context.Context, owner string) ([]*ActivityEntry, error) {
	return db.findActivity(ctx, bson.M{"owner": owner}, 0)
}

// findActivity is the single owner-scoped read: like courses, there is no method
// that reads the log without an owner filter. A limit <= 0 reads without a cap.
func (db *DB) findActivity(ctx context.Context, filter bson.M, limit int64) ([]*ActivityEntry, error) {
	opts := options.Find().SetSort(bson.D{{Key: "at", Value: -1}})
	if limit > 0 {
		opts.SetLimit(limit)
	}
	cur, err := db.collection(collectionActivity).Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("cannot read activity: %w", err)
	}
	var out []*ActivityEntry
	if err := cur.All(ctx, &out); err != nil {
		return nil, fmt.Errorf("cannot decode activity: %w", err)
	}
	return out, nil
}
