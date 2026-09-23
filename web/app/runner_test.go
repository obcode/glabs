package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/obcode/glabs/v3/web/db"
)

// A job that is already past its grace window is expired, not run.
func TestRunJob_expiredWhenPastGrace(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}

	job := &db.ScheduledJob{
		ID: "j1", Owner: owner, Op: "setaccess", Course: "uc", Assignment: "blatt1",
		RunAt: time.Now().Add(-2 * time.Hour), GraceMin: 60, ConfigHash: "irrelevant",
		Status: db.JobRunning,
	}
	fs.jobs["j1"] = job

	a.runJob(context.Background(), job)

	if fs.jobs["j1"].Status != db.JobExpired {
		t.Errorf("status = %q, want expired", fs.jobs["j1"].Status)
	}
	if len(fs.activity) == 0 || fs.activity[len(fs.activity)-1].Status != "failed" {
		t.Error("an expired job should be mirrored into the activity log as failed")
	}
}

// A job whose config drifted since scheduling (hash mismatch) fails instead of
// doing something unintended.
func TestRunJob_failsOnConfigDrift(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}

	job := &db.ScheduledJob{
		ID: "j1", Owner: owner, Op: "setaccess", Course: "uc", Assignment: "blatt1",
		RunAt: time.Now(), GraceMin: 60, ConfigHash: "stale-hash", Status: db.JobRunning,
	}
	fs.jobs["j1"] = job

	a.runJob(context.Background(), job)

	if fs.jobs["j1"].Status != db.JobFailed {
		t.Errorf("status = %q, want failed", fs.jobs["j1"].Status)
	}
	if !strings.Contains(fs.jobs["j1"].Err, "changed") {
		t.Errorf("err = %q, want a config-changed message", fs.jobs["j1"].Err)
	}
}

// With the right hash but no stored GitLab token, the job fails at client
// construction — before any GitLab call.
func TestRunJob_failsWithoutStoredToken(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}

	cfg, err := a.resolveAssignmentConfig(ctxAs(owner), "uc", "blatt1")
	if err != nil || cfg == nil {
		t.Fatalf("resolve: cfg=%v err=%v", cfg, err)
	}
	hash, err := configHash(cfg)
	if err != nil {
		t.Fatalf("configHash: %v", err)
	}

	job := &db.ScheduledJob{
		ID: "j1", Owner: owner, Op: "setaccess", Course: "uc", Assignment: "blatt1",
		RunAt: time.Now(), GraceMin: 60, ConfigHash: hash, Status: db.JobRunning,
	}
	fs.jobs["j1"] = job

	a.runJob(context.Background(), job)

	if fs.jobs["j1"].Status != db.JobFailed {
		t.Errorf("status = %q, want failed", fs.jobs["j1"].Status)
	}
	if fs.jobs["j1"].Err == "" {
		t.Error("a job that cannot authenticate should record an error")
	}
}

// The poll loop leaves a not-yet-due job untouched.
func TestRunDueJobs_skipsFutureJobs(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	a := &App{db: fs, gitlabHost: "https://gl"}

	fs.jobs["future"] = &db.ScheduledJob{
		ID: "future", Owner: owner, Op: "setaccess", Course: "uc", Assignment: "blatt1",
		RunAt: time.Now().Add(time.Hour), GraceMin: 60, Status: db.JobPending,
	}

	a.runDueJobs(context.Background(), "worker-1")

	if fs.jobs["future"].Status != db.JobPending {
		t.Errorf("a future job was claimed (status %q), want it left pending", fs.jobs["future"].Status)
	}
}

// TestRunnerReapsOnceAnHour covers what replaced MongoDB's TTL indexes.
//
// Retention used to be the database's business: an index option, invisible in
// the log and impossible to test. Now it is a call on the runner's tick, which
// makes two things worth pinning -- that it happens at all, and that it does not
// happen on every one of the 120 ticks an hour.
func TestRunnerReapsOnceAnHour(t *testing.T) {
	fs := newFakeStore()
	a := &App{db: fs, ops: newOpGuard()}

	now := time.Now()
	// One finished job well past the 30-day retention, and one finished
	// yesterday. And an event past the 180-day one.
	old := now.Add(-40 * 24 * time.Hour)
	recent := now.Add(-24 * time.Hour)
	fs.jobs["old"] = &db.ScheduledJob{
		ID: "old", Owner: "a@hm.edu", Status: db.JobDone, FinishedAt: &old,
	}
	fs.jobs["recent"] = &db.ScheduledJob{
		ID: "recent", Owner: "a@hm.edu", Status: db.JobDone, FinishedAt: &recent,
	}
	// A job scheduled long ago that never ran. Reaping it would silently cancel
	// work somebody is still waiting for.
	fs.jobs["pending"] = &db.ScheduledJob{
		ID: "pending", Owner: "a@hm.edu", Status: db.JobPending,
		RunAt: now.Add(-400 * 24 * time.Hour),
	}
	fs.events = []*db.Event{
		{At: now.Add(-200 * 24 * time.Hour), Type: db.EventLogin},
		{At: now.Add(-time.Hour), Type: db.EventLogin},
	}

	a.reapExpired(t.Context(), now)

	if fs.reapCalls != 1 {
		t.Errorf("ReapExpired was called %d times, want 1", fs.reapCalls)
	}
	if _, ok := fs.jobs["old"]; ok {
		t.Error("a job finished 40 days ago survived the sweep")
	}
	if _, ok := fs.jobs["recent"]; !ok {
		t.Error("a job finished yesterday was reaped, want it kept for 30 days")
	}
	if _, ok := fs.jobs["pending"]; !ok {
		t.Error("a PENDING job was reaped — the sweep must only ever touch finished ones")
	}
	if len(fs.events) != 1 {
		t.Errorf("%d events left, want 1", len(fs.events))
	}
}

// TestRunnerSurvivesAFailingReap pins that housekeeping cannot stop the work
// people are waiting for: a database error while reaping is logged and the tick
// carries on.
func TestRunnerSurvivesAFailingReap(t *testing.T) {
	fs := &failingReapStore{fakeStore: newFakeStore()}
	a := &App{db: fs, ops: newOpGuard()}

	// The bar is simply that this returns rather than panicking or propagating.
	a.reapExpired(t.Context(), time.Now())

	if fs.reapCalls != 1 {
		t.Errorf("ReapExpired was called %d times, want 1", fs.reapCalls)
	}
}

type failingReapStore struct {
	*fakeStore
}

func (f *failingReapStore) ReapExpired(_ context.Context, _ time.Time) (int64, int64, error) {
	f.reapCalls++
	return 0, 0, errors.New("the database went away")
}
