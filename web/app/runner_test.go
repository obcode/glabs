package app

import (
	"context"
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
