// Package migrate is the one-off import of glabs-web's data from MongoDB into
// PostgreSQL. It exists for a single evening and is deleted with the MongoDB
// store afterwards.
//
// It is a subcommand of glabs-web rather than a separate module or binary, for
// one practical reason: the production host has Docker and nothing else. No Go
// toolchain, no psql, no mongosh outside the containers. The image that is
// already on the host is therefore the only thing that can be run there, and
// `docker compose run --rm glabs-web mongo2pg` needs no new machinery at all.
// (plexams solved the same problem differently -- its tool reads dump files from
// a laptop -- because its backup format is a directory of .bson files, while
// glabs' is a single mongodump archive.)
//
// Reads go through the raw driver rather than through db.DB, because the store's
// API is owner-scoped by design: there is deliberately no method that reads
// every user's courses. Decoding uses the same DTOs the application uses, so a
// document lands in exactly the struct the server would have read, and writes go
// through the same typed db.PG methods the server writes with. That is the whole
// reason this is a Go program and not a pile of hand-written INSERTs.
package migrate

import (
	"context"
	"errors"
	"fmt"

	"github.com/obcode/glabs/v3/web/db"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// source is everything MongoDB holds, decoded into the application's own types.
//
// All of it at once, in memory: the whole database is a handful of courses, a
// rolling 180-day event window and one row per lecturer. Streaming would buy
// nothing and would make the verification -- which compares both sides
// field by field -- impossible to write.
type source struct {
	State    *db.SystemState
	Secrets  []*db.UserSecret
	Courses  []*db.StoredCourse
	Jobs     []*db.ScheduledJob
	Activity []*db.ActivityEntry
	Events   []*db.Event
}

// Counts reports the same per-table numbers as PG.CountsForImport, so the two
// sides can be compared with one map against another.
func (s *source) Counts() map[string]int64 {
	state := int64(0)
	if s.State != nil {
		state = 1
	}
	return map[string]int64{
		"system_state":   state,
		"user_secrets":   int64(len(s.Secrets)),
		"courses":        int64(len(s.Courses)),
		"scheduled_jobs": int64(len(s.Jobs)),
		"activity":       int64(len(s.Activity)),
		"events":         int64(len(s.Events)),
	}
}

// readAll loads every collection.
//
// The sort orders are not cosmetic: they make two runs produce the same order,
// so a diff between a rehearsal and the real thing is a diff in the data rather
// than in the listing.
func readAll(ctx context.Context, client *mongo.Client, database string) (*source, error) {
	mdb := client.Database(database)
	var s source

	// system: one document with a fixed id, and its absence is normal -- a server
	// that has never sent a summary has never written it.
	var state db.SystemState
	err := mdb.Collection("system").FindOne(ctx, bson.M{"_id": "state"}).Decode(&state)
	switch {
	case err == nil:
		s.State = &state
	case errors.Is(err, mongo.ErrNoDocuments):
		// Normal: a server that has never sent a summary never wrote it.
		s.State = nil
	default:
		return nil, fmt.Errorf("cannot read system state: %w", err)
	}

	if err := readInto(ctx, mdb, "user_secrets", bson.D{{Key: "owner", Value: 1}}, &s.Secrets); err != nil {
		return nil, err
	}
	if err := readInto(ctx, mdb, "courses",
		bson.D{{Key: "owner", Value: 1}, {Key: "name", Value: 1}}, &s.Courses); err != nil {
		return nil, err
	}
	if err := readInto(ctx, mdb, "scheduled_jobs", bson.D{{Key: "_id", Value: 1}}, &s.Jobs); err != nil {
		return nil, err
	}
	if err := readInto(ctx, mdb, "activity",
		bson.D{{Key: "at", Value: 1}, {Key: "owner", Value: 1}}, &s.Activity); err != nil {
		return nil, err
	}
	if err := readInto(ctx, mdb, "events", bson.D{{Key: "at", Value: 1}}, &s.Events); err != nil {
		return nil, err
	}

	return &s, nil
}

func readInto[T any](ctx context.Context, mdb *mongo.Database, collection string, sort bson.D, out *[]*T) error {
	cur, err := mdb.Collection(collection).Find(ctx, bson.M{}, options.Find().SetSort(sort))
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", collection, err)
	}
	if err := cur.All(ctx, out); err != nil {
		// A decode error here is the most valuable thing this tool can find, and
		// the reason for --dry-run: a document the application's own struct cannot
		// read is a document that would be silently lost.
		return fmt.Errorf("cannot decode %s: %w", collection, err)
	}
	return nil
}

// owners returns every owner that appears anywhere, so the verification can read
// back through the store's owner-scoped API.
func (s *source) owners() []string {
	seen := map[string]bool{}
	add := func(o string) {
		if o != "" {
			seen[o] = true
		}
	}
	for _, x := range s.Secrets {
		add(x.Owner)
	}
	for _, x := range s.Courses {
		add(x.Owner)
	}
	for _, x := range s.Jobs {
		add(x.Owner)
	}
	for _, x := range s.Activity {
		add(x.Owner)
	}

	out := make([]string, 0, len(seen))
	for o := range seen {
		out = append(out, o)
	}
	return out
}
