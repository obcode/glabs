package app

import (
	"context"
	"fmt"
	"time"

	"github.com/obcode/glabs/v3/web/db"
)

// defaultGraceMinutes is how long after runAt a job may still start before the
// runner gives up and marks it expired (rather than firing a deadline job hours
// late). Overridable per job.
const defaultGraceMinutes = 60

// ScheduleOp queues a mutating operation to run at runAt. It reuses the SAME
// confirm token as RunOp: planning, running now, and scheduling for later are one
// flow. The token proves the plan is fresh (it carries the config hash, re-checked
// when the job fires) and names the operation, so scheduling needs no new
// preview. Scheduling presupposes a stored GitLab token — the runner has no way to
// ask for one later.
func (a *App) ScheduleOp(ctx context.Context, token string, runAt time.Time, graceMinutes *int, confirmPhrase string) (*db.ScheduledJob, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}

	tok, err := a.openOpToken(token)
	if err != nil {
		return nil, err
	}
	if tok.Owner != o {
		return nil, fmt.Errorf("this plan was not created by you")
	}
	// Destructive ops keep the same confirm-phrase gate as running now: the
	// deliberate phrase is the human confirmation, since no one watches at fire time.
	if isDestructiveOp(tok.Op) {
		want := tok.Course + "/" + tok.Assignment
		if confirmPhrase != want {
			return nil, fmt.Errorf("type %q to confirm scheduling this destructive operation", want)
		}
	}

	// A stored PAT is required up front — a job that can never authenticate should
	// not be accepted in the first place.
	secret, err := a.db.GetUserSecret(ctx, o)
	if err != nil {
		return nil, err
	}
	if secret == nil || secret.GitLab == nil {
		return nil, fmt.Errorf("store a GitLab token before scheduling operations")
	}

	grace := defaultGraceMinutes
	if graceMinutes != nil {
		if grace = *graceMinutes; grace < 0 {
			grace = 0
		}
	}

	job := &db.ScheduledJob{
		Owner:      o,
		Op:         tok.Op,
		Course:     tok.Course,
		Assignment: tok.Assignment,
		OnlyFor:    tok.OnlyFor,
		Params:     tok.Params,
		RunAt:      runAt.UTC(),
		ConfigHash: tok.ConfigHash,
		Status:     db.JobPending,
		GraceMin:   grace,
		CreatedAt:  time.Now(),
	}
	if err := a.db.SaveJob(ctx, job); err != nil {
		return nil, err
	}
	a.sendScheduledConfirmation(job)
	return job, nil
}

// CancelScheduledJob cancels one of the caller's pending jobs.
func (a *App) CancelScheduledJob(ctx context.Context, id string) (*db.ScheduledJob, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	return a.db.CancelJob(ctx, o, id)
}

// ScheduledJobs returns the caller's jobs, newest scheduled first, optionally
// filtered to the given statuses.
func (a *App) ScheduledJobs(ctx context.Context, statuses []string) ([]*db.ScheduledJob, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	return a.db.JobsOf(ctx, o, statuses)
}

// ScheduledJob returns one of the caller's jobs.
func (a *App) ScheduledJob(ctx context.Context, id string) (*db.ScheduledJob, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	return a.db.JobOf(ctx, o, id)
}
