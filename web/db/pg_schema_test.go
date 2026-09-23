package db_test

import (
	"testing"
	"time"

	"github.com/obcode/glabs/v3/internal/pgtest"
	"github.com/obcode/glabs/v3/web/db"
)

// TestPGStoresSQLNullNotJSONNull pins that an absent map becomes SQL NULL rather
// than a jsonb `null`.
//
// The store contract suite cannot see the difference: json.Marshal of a nil map
// produces the bytes `null`, and decoding those back yields a nil map again, so
// the round trip is identical either way. The database is not indifferent. A
// jsonb null makes the column NOT NULL-but-containing-null, which is a different
// answer to `where params is null` and to any partial index built on it later.
//
// Verified by mutation: removing the nil guards in pg_json.go leaves the
// contract suite green and turns this test red. That is the whole reason it
// exists separately.
func TestPGStoresSQLNullNotJSONNull(t *testing.T) {
	pg := pgtest.NewDB(t)
	ctx := t.Context()

	if err := pg.RecordActivity(ctx, &db.ActivityEntry{
		Owner: "a@hm.edu", Course: "fopra", Assignment: "blatt01",
		Op: "archive", Status: "done", At: time.Now(), Params: nil,
	}); err != nil {
		t.Fatalf("RecordActivity: %v", err)
	}

	job := &db.ScheduledJob{
		Owner: "a@hm.edu", Op: "setaccess", Course: "fopra", Assignment: "blatt01",
		RunAt: time.Now(), Status: db.JobPending, CreatedAt: time.Now(),
		Params: nil, OnlyFor: nil,
	}
	if err := pg.SaveJob(ctx, job); err != nil {
		t.Fatalf("SaveJob: %v", err)
	}

	for _, want := range []struct {
		what  string
		query string
	}{
		{"activity.params", "select params is null from activity limit 1"},
		{"scheduled_jobs.params", "select params is null from scheduled_jobs limit 1"},
		{"scheduled_jobs.only_for", "select only_for is null from scheduled_jobs limit 1"},
	} {
		var isNull bool
		if err := pg.PoolForTest().QueryRow(ctx, want.query).Scan(&isNull); err != nil {
			t.Fatalf("%s: %v", want.what, err)
		}
		if !isNull {
			t.Errorf("%s is not SQL NULL — an absent value was stored as a jsonb null or an empty array", want.what)
		}
	}
}

// TestPGRefusesHalfATokenAndASecondSystemRow checks the two constraints that
// make an invalid state unrepresentable rather than merely never written.
//
// Neither is reachable through the store's own methods, which is exactly why
// they are worth having and why the test has to go around them: they are what
// protects the data from a future method, a manual fix during an incident, or
// the one-off migration tool.
func TestPGRefusesHalfATokenAndASecondSystemRow(t *testing.T) {
	pg := pgtest.NewDB(t)
	ctx := t.Context()
	pool := pg.PoolForTest()

	// Three of the four gitlab columns set: Mongo's $set/$unset pair could
	// produce this; the CHECK cannot.
	_, err := pool.Exec(ctx,
		`insert into user_secrets (owner, gitlab_key_version, gitlab_nonce, gitlab_ciphertext)
		 values ('a@hm.edu', 1, '\x00'::bytea, '\x01'::bytea)`)
	if err == nil {
		t.Error("a half-written GitLab token was accepted, want the all-or-nothing CHECK to reject it")
	}

	// The singleton really is one.
	if _, err := pool.Exec(ctx, `insert into system_state (id) values ('zweite')`); err == nil {
		t.Error("a second system_state row was accepted, want the id CHECK to reject it")
	}
}

// TestPGReapExpiredRemovesOnlyWhatIsOldAndFinished covers the replacement for
// MongoDB's TTL indexes. There is no counterpart on the Mongo store -- the TTL
// monitor is the database's business there -- so this cannot live in the shared
// contract suite.
func TestPGReapExpiredRemovesOnlyWhatIsOldAndFinished(t *testing.T) {
	pg := pgtest.NewDB(t)
	ctx := t.Context()
	now := time.Now()

	// A finished job older than the 30-day retention, and one just inside it.
	old := &db.ScheduledJob{
		Owner: "a@hm.edu", Op: "setaccess", Course: "fopra", Assignment: "blatt01",
		RunAt: now.Add(-40 * 24 * time.Hour), Status: db.JobDone,
		CreatedAt: now.Add(-40 * 24 * time.Hour),
	}
	recent := &db.ScheduledJob{
		Owner: "a@hm.edu", Op: "setaccess", Course: "fopra", Assignment: "blatt02",
		RunAt: now.Add(-24 * time.Hour), Status: db.JobDone, CreatedAt: now.Add(-24 * time.Hour),
	}
	// A job scheduled far in the past that never ran. It has no finished_at, and
	// must survive: reaping a pending job would silently cancel it.
	pending := &db.ScheduledJob{
		Owner: "a@hm.edu", Op: "setaccess", Course: "fopra", Assignment: "blatt03",
		RunAt: now.Add(-400 * 24 * time.Hour), Status: db.JobPending,
		CreatedAt: now.Add(-400 * 24 * time.Hour),
	}
	for _, j := range []*db.ScheduledJob{old, recent, pending} {
		if err := pg.SaveJob(ctx, j); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}
	}
	// FinishJob is what sets finished_at, and it sets it to now — so the two
	// finished jobs are aged with raw SQL instead.
	for _, j := range []*db.ScheduledJob{old, recent} {
		age := -40 * 24 * time.Hour
		if j == recent {
			age = -24 * time.Hour
		}
		if _, err := pg.PoolForTest().Exec(ctx,
			`update scheduled_jobs set finished_at = $2 where id = $1`, j.ID, now.Add(age)); err != nil {
			t.Fatalf("cannot age job: %v", err)
		}
	}

	for _, at := range []time.Time{now.Add(-200 * 24 * time.Hour), now.Add(-time.Hour)} {
		if err := pg.RecordEvent(ctx, &db.Event{At: at, Type: db.EventLogin, Severity: db.SeverityInfo}); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}

	jobs, events, err := pg.ReapExpired(ctx, now)
	if err != nil {
		t.Fatalf("ReapExpired: %v", err)
	}
	if jobs != 1 {
		t.Errorf("reaped %d jobs, want 1", jobs)
	}
	if events != 1 {
		t.Errorf("reaped %d events, want 1", events)
	}

	left, err := pg.JobsOf(ctx, "a@hm.edu", nil)
	if err != nil {
		t.Fatalf("JobsOf: %v", err)
	}
	survived := map[string]bool{}
	for _, j := range left {
		survived[j.Assignment] = true
	}
	if survived["blatt01"] {
		t.Error("the 40-day-old finished job survived the sweep")
	}
	if !survived["blatt02"] {
		t.Error("a job finished yesterday was reaped, want it kept for 30 days")
	}
	if !survived["blatt03"] {
		t.Error("a PENDING job was reaped — the sweep must only ever touch finished ones")
	}

	remaining, err := pg.RecentEvents(ctx, now.Add(-365*24*time.Hour), 0)
	if err != nil {
		t.Fatalf("RecentEvents: %v", err)
	}
	if len(remaining) != 1 {
		t.Errorf("%d events left, want 1", len(remaining))
	}
}
