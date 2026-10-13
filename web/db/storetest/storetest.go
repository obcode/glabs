// Package storetest is the contract a glabs-web store implementation has to
// satisfy, written once and run against it.
//
// It exists because web/db had no tests at all when the move from MongoDB to
// PostgreSQL began. Porting a database layer without one means testing the new
// implementation against one's own recollection of the old one rather than
// against what production actually does — and the parts most easily
// misremembered (does a delete of someone else's course fail or silently
// succeed? does an absent secret come back as an error or as nil?) are exactly
// the parts the app above relies on.
//
// So it was written against the MongoDB implementation FIRST, while that was
// still the one in production, and only then pointed at PostgreSQL. Every
// assertion is derived from reading the old implementation, not from taste.
// **Only PostgreSQL remains** since 2026-10-13, which is worth being honest
// about: the suite no longer proves that two implementations agree, which was
// the point of it. What it still does is hold PostgreSQL to the behaviour the
// app was written against — so an assertion here that looks arbitrary is
// probably load-bearing, and the comment usually says why.
//
// Two things it deliberately does NOT assert, both inherited from needing to be
// green against both implementations and both still sensible:
//
//   - the order of records sharing a timestamp. MongoDB left those in an
//     arbitrary order; PostgreSQL breaks the tie on the identity column. The
//     suite only ever compares sets there.
//   - whether an empty result is a nil slice or an empty one. The callers in
//     web/app all use len(), the two drivers differed, and pinning it would
//     freeze a detail nobody depends on.
package storetest

import (
	"context"
	"testing"
	"time"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/secrets"
)

// Store is the slice of the database the application uses. It is a copy of the
// unexported `store` interface in web/app — deliberately a copy rather than a
// shared type, because that one is the app's statement of what it needs and this
// one is the storage layer's statement of what it promises. They happen to
// coincide today.
//
// ReapExpired is NOT part of it: MongoDB did that with a TTL index rather than a
// method, so the interface never carried one. It is tested separately, by the
// tests of the PostgreSQL implementation.
type Store interface {
	CoursesOf(ctx context.Context, owner string) ([]*db.StoredCourse, error)
	CourseOf(ctx context.Context, owner, name string) (*db.StoredCourse, error)
	SaveCourse(ctx context.Context, course *db.StoredCourse) error
	DeleteCourse(ctx context.Context, owner, name string) error

	GetUserSecret(ctx context.Context, owner string) (*db.UserSecret, error)
	SaveUserGitLabToken(ctx context.Context, owner string, sealed secrets.SealedValue, updatedAt time.Time) error
	DeleteUserGitLabToken(ctx context.Context, owner string) error

	RecordActivity(ctx context.Context, e *db.ActivityEntry) error
	ActivityFor(ctx context.Context, owner, course, assignment string) ([]*db.ActivityEntry, error)
	CourseActivityFor(ctx context.Context, owner, course string) ([]*db.ActivityEntry, error)
	AllActivityFor(ctx context.Context, owner string) ([]*db.ActivityEntry, error)

	SaveJob(ctx context.Context, job *db.ScheduledJob) error
	CancelJob(ctx context.Context, owner, id string) (*db.ScheduledJob, error)
	JobsOf(ctx context.Context, owner string, statuses []string) ([]*db.ScheduledJob, error)
	JobOf(ctx context.Context, owner, id string) (*db.ScheduledJob, error)
	ClaimDueJob(ctx context.Context, workerID string, now time.Time) (*db.ScheduledJob, error)
	FinishJob(ctx context.Context, id, status, logText, errText string) error
	UnnotifiedTerminalJobs(ctx context.Context) ([]*db.ScheduledJob, error)
	MarkNotified(ctx context.Context, id string) error

	RecordEvent(ctx context.Context, e *db.Event) error
	EventsBetween(ctx context.Context, since, until time.Time) ([]*db.Event, error)
	RecentEvents(ctx context.Context, since time.Time, limit int64) ([]*db.Event, error)

	SystemState(ctx context.Context) (*db.SystemState, error)
	SetSummarySentAt(ctx context.Context, at time.Time) error
}

// NewStore hands back a store with no data in it. It is called once per subtest,
// so nothing a case writes can reach the next one — the alternative, cleaning up
// afterwards, leaves the suite's correctness depending on every case remembering
// to do so.
type NewStore func(t *testing.T) Store

// Run executes the whole contract against one implementation.
//
// The groups mirror the files in web/db, so a failure names the file to open.
func Run(t *testing.T, newStore NewStore) {
	t.Helper()

	t.Run("courses", func(t *testing.T) { runCourses(t, newStore) })
	t.Run("user_secrets", func(t *testing.T) { runUserSecrets(t, newStore) })
	t.Run("activity", func(t *testing.T) { runActivity(t, newStore) })
	t.Run("jobs", func(t *testing.T) { runJobs(t, newStore) })
	t.Run("events", func(t *testing.T) { runEvents(t, newStore) })
	t.Run("system", func(t *testing.T) { runSystem(t, newStore) })
	t.Run("timezone", func(t *testing.T) { runTimezone(t, newStore) })
}
