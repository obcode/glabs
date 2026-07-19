package app

import (
	"testing"
	"time"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/secrets"
)

// withStoredToken gives the owner a (dummy) stored GitLab token so scheduling,
// which requires one, is allowed.
func withStoredToken(fs *fakeStore, owner string) {
	fs.userSecret[owner] = &db.UserSecret{Owner: owner, GitLab: &secrets.SealedValue{}}
}

func TestScheduleOp_planTokenToPendingJob(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	withStoredToken(fs, owner)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}
	ctx := ctxAs(owner)

	plan, err := a.PlanOp(ctx, "setaccess", "uc", "blatt1", nil, nil)
	if err != nil {
		t.Fatalf("PlanOp: %v", err)
	}

	runAt := time.Now().Add(time.Hour)
	job, err := a.ScheduleOp(ctx, plan.Token, runAt, nil, "")
	if err != nil {
		t.Fatalf("ScheduleOp: %v", err)
	}
	if job.Status != db.JobPending {
		t.Errorf("status = %q, want pending", job.Status)
	}
	if job.Op != "setaccess" || job.Course != "uc" || job.Assignment != "blatt1" {
		t.Errorf("job header wrong: %+v", job)
	}
	if job.GraceMin != defaultGraceMinutes {
		t.Errorf("grace = %d, want default %d", job.GraceMin, defaultGraceMinutes)
	}
	if job.ConfigHash == "" {
		t.Error("job is missing the config hash carried by the token")
	}
	if !job.RunAt.Equal(runAt.UTC()) {
		t.Errorf("runAt = %v, want %v (UTC)", job.RunAt, runAt.UTC())
	}

	// Listing: unfiltered finds it; a status filter includes/excludes correctly.
	all, err := a.ScheduledJobs(ctx, nil)
	if err != nil || len(all) != 1 {
		t.Fatalf("ScheduledJobs = %d (%v), want 1", len(all), err)
	}
	if pend, _ := a.ScheduledJobs(ctx, []string{db.JobPending}); len(pend) != 1 {
		t.Errorf("pending filter = %d, want 1", len(pend))
	}
	if done, _ := a.ScheduledJobs(ctx, []string{db.JobDone}); len(done) != 0 {
		t.Errorf("done filter = %d, want 0", len(done))
	}

	// Get by id.
	got, err := a.ScheduledJob(ctx, job.ID)
	if err != nil || got.ID != job.ID {
		t.Fatalf("ScheduledJob = %v (%v)", got, err)
	}

	// Cancel a pending job; cancelling again fails (no longer pending).
	cancelled, err := a.CancelScheduledJob(ctx, job.ID)
	if err != nil || cancelled.Status != db.JobCancelled {
		t.Fatalf("cancel = %v (%v)", cancelled, err)
	}
	if _, err := a.CancelScheduledJob(ctx, job.ID); err == nil {
		t.Error("cancelling an already-cancelled job should fail")
	}
}

func TestScheduleOp_requiresStoredGitLabToken(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}
	ctx := ctxAs(owner)

	plan, err := a.PlanOp(ctx, "setaccess", "uc", "blatt1", nil, nil)
	if err != nil {
		t.Fatalf("PlanOp: %v", err)
	}
	if _, err := a.ScheduleOp(ctx, plan.Token, time.Now().Add(time.Hour), nil, ""); err == nil {
		t.Error("scheduling without a stored GitLab token should fail")
	}
}

func TestScheduleOp_destructiveNeedsConfirmPhrase(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	withStoredToken(fs, owner)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}
	ctx := ctxAs(owner)

	plan, err := a.PlanOp(ctx, "delete", "uc", "blatt1", nil, nil)
	if err != nil {
		t.Fatalf("PlanOp: %v", err)
	}
	future := time.Now().Add(time.Hour)
	if _, err := a.ScheduleOp(ctx, plan.Token, future, nil, ""); err == nil {
		t.Error("scheduling a destructive op without the confirm phrase should fail")
	}
	job, err := a.ScheduleOp(ctx, plan.Token, future, nil, "uc/blatt1")
	if err != nil {
		t.Fatalf("ScheduleOp with phrase: %v", err)
	}
	if job.Op != "delete" || job.Status != db.JobPending {
		t.Errorf("job wrong: %+v", job)
	}
}

// A job belongs to its owner; another user can neither see nor cancel it.
func TestScheduledJobs_ownerIsolation(t *testing.T) {
	fs := newFakeStore()
	fs.courses["a@hm.edu/uc"] = storedCourse(t, "a@hm.edu", urlsCourse)
	withStoredToken(fs, "a@hm.edu")
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}

	plan, err := a.PlanOp(ctxAs("a@hm.edu"), "setaccess", "uc", "blatt1", nil, nil)
	if err != nil {
		t.Fatalf("PlanOp: %v", err)
	}
	job, err := a.ScheduleOp(ctxAs("a@hm.edu"), plan.Token, time.Now().Add(time.Hour), nil, "")
	if err != nil {
		t.Fatalf("ScheduleOp: %v", err)
	}

	// User B sees no jobs and cannot cancel A's job.
	if jobs, _ := a.ScheduledJobs(ctxAs("b@hm.edu"), nil); len(jobs) != 0 {
		t.Errorf("user B sees %d of A's jobs, want 0", len(jobs))
	}
	if _, err := a.CancelScheduledJob(ctxAs("b@hm.edu"), job.ID); err == nil {
		t.Error("user B cancelling A's job should fail")
	}
}
