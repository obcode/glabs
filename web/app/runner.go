package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/obcode/glabs/v3/reporter"
	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/graph/model"
	"github.com/obcode/glabs/v3/web/principal"
	"github.com/rs/zerolog/log"
)

// jobPollInterval is how often the runner looks for due jobs. Scheduled times are
// absolute wall-clock instants, so a coarse poll is fine.
const jobPollInterval = 30 * time.Second

// StartJobRunner runs the scheduled-job poll loop until ctx is cancelled. Each
// tick it drains every due job, claiming them one at a time so exactly one runner
// ever owns a given job. It is safe to run one per instance against a shared
// database. Jobs run synchronously in the loop, so a shutdown between ticks waits
// for the current job to finish.
func (a *App) StartJobRunner(ctx context.Context) {
	worker := workerID()
	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()
	log.Info().Str("worker", worker).Dur("interval", jobPollInterval).Msg("scheduled-job runner started")
	for {
		a.runDueJobs(ctx, worker)
		a.notifyFinishedJobs(ctx)
		select {
		case <-ctx.Done():
			log.Info().Msg("scheduled-job runner stopped")
			return
		case <-ticker.C:
		}
	}
}

func workerID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	return fmt.Sprintf("%s/%d", host, os.Getpid())
}

// runDueJobs claims and runs due jobs one at a time until none remain (so a batch
// that came due during downtime is caught up in one tick).
func (a *App) runDueJobs(ctx context.Context, worker string) {
	for ctx.Err() == nil {
		job, err := a.db.ClaimDueJob(ctx, worker, time.Now())
		if errors.Is(err, db.ErrNoDueJob) {
			return
		}
		if err != nil {
			log.Error().Err(err).Msg("cannot claim a due job")
			return
		}
		a.runJob(ctx, job)
	}
}

// runJob executes one claimed job to a terminal state. It is the guarded core: a
// panic is recovered (so one bad job never takes the server down), the grace
// window turns a too-late job into `expired`, and the plan's config hash is
// re-checked so a job whose config drifted since scheduling fails rather than
// doing something unintended.
func (a *App) runJob(parent context.Context, job *db.ScheduledJob) {
	// Detached context so a shutdown mid-operation does not cancel the GitLab
	// calls; the runner acts as the job's owner (its context principal), never as a
	// request user.
	ctx := principal.WithUser(context.WithoutCancel(parent), &model.User{Email: job.Owner})

	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("job", job.ID).Msg("scheduled job panicked")
			a.finishJob(ctx, job, db.JobFailed, "", fmt.Sprintf("internal error: %v", r))
		}
	}()

	// Grace window: past runAt + grace, do not fire a stale deadline job.
	if time.Now().After(job.RunAt.Add(time.Duration(job.GraceMin) * time.Minute)) {
		a.finishJob(ctx, job, db.JobExpired, "",
			fmt.Sprintf("grace window of %d min passed (was due %s)", job.GraceMin, job.RunAt.UTC().Format(time.RFC3339)))
		return
	}

	// Re-resolve and compare the config hash: reject a job whose config changed.
	cfg, err := a.resolveAssignmentConfig(ctx, job.Course, job.Assignment, job.OnlyFor...)
	if err != nil {
		a.finishJob(ctx, job, db.JobFailed, "", err.Error())
		return
	}
	if cfg == nil {
		a.finishJob(ctx, job, db.JobFailed, "",
			fmt.Sprintf("assignment %q of course %q no longer resolves", job.Assignment, job.Course))
		return
	}
	hash, err := configHash(cfg)
	if err != nil {
		a.finishJob(ctx, job, db.JobFailed, "", err.Error())
		return
	}
	if hash != job.ConfigHash {
		a.finishJob(ctx, job, db.JobFailed, "",
			"the configuration changed since the job was scheduled — it was not run")
		return
	}
	if cfg.Seeder != nil {
		a.finishJob(ctx, job, db.JobFailed, "",
			"assignment configures a seeder, which the web cannot run")
		return
	}

	rep := &captureReporter{}
	client, err := a.gitlabClientFor(ctx, job.Owner, rep)
	if err != nil {
		a.finishJob(ctx, job, db.JobFailed, rep.String(), err.Error())
		return
	}

	tok := &opToken{Op: job.Op, Course: job.Course, Assignment: job.Assignment, Params: job.Params, OnlyFor: job.OnlyFor}
	rep.line(fmt.Sprintf("running %s on %s/%s (%d repositories)", job.Op, job.Course, job.Assignment, len(cfg.RepoTargets())))
	if err := executeOp(client, tok, cfg); err != nil {
		a.finishJob(ctx, job, db.JobFailed, rep.String(), err.Error())
		return
	}
	a.finishJob(ctx, job, db.JobDone, rep.String(), "")
}

// finishJob records a job's terminal state and mirrors it into the activity log so
// the course page shows a scheduled run exactly like an interactive one.
func (a *App) finishJob(ctx context.Context, job *db.ScheduledJob, status, logText, errText string) {
	if err := a.db.FinishJob(ctx, job.ID, status, logText, errText); err != nil {
		log.Error().Err(err).Str("job", job.ID).Msg("cannot record scheduled-job outcome")
	}
	activityStatus, detail := "done", "scheduled run completed"
	if status != db.JobDone {
		activityStatus, detail = "failed", errText
	}
	tok := &opToken{Op: job.Op, Course: job.Course, Assignment: job.Assignment, Params: job.Params, OnlyFor: job.OnlyFor}
	if err := a.recordOp(ctx, job.Owner, tok, activityStatus, detail); err != nil {
		log.Warn().Err(err).Str("job", job.ID).Msg("cannot record scheduled-job activity")
	}
	a.recordEvent(ctx, &db.Event{
		Type:       jobEventType(status),
		Actor:      job.Owner,
		Course:     job.Course,
		Assignment: job.Assignment,
		Op:         job.Op,
		Severity:   jobEventSeverity(status),
		Detail:     detail,
		JobID:      job.ID,
	})
}

// jobEventType maps a terminal job status to its monitoring event type.
func jobEventType(status string) string {
	switch status {
	case db.JobDone:
		return db.EventJobDone
	case db.JobExpired:
		return db.EventJobExpired
	case db.JobCancelled:
		return db.EventJobCancelled
	default:
		return db.EventJobFailed
	}
}

// jobEventSeverity flags failed/expired job outcomes so the digest and admin page
// can surface them; a normal completion is info.
func jobEventSeverity(status string) string {
	if status == db.JobDone {
		return db.SeverityInfo
	}
	return db.SeverityError
}

// captureReporter is a reporter.Reporter that accumulates the operation's output
// into a string, so a scheduled run (which no one watches) still keeps a log on
// the job document.
type captureReporter struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (c *captureReporter) line(s string) {
	s = strings.TrimSpace(ansiRE.ReplaceAllString(s, ""))
	if s == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.buf.WriteString(s)
	c.buf.WriteByte('\n')
}

func (c *captureReporter) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

func (c *captureReporter) Printf(format string, a ...any) { c.line(fmt.Sprintf(format, a...)) }
func (c *captureReporter) Println(a ...any)               { c.line(fmt.Sprintln(a...)) }
func (c *captureReporter) Task(description string) reporter.Task {
	c.line(description)
	return &captureTask{c}
}

type captureTask struct{ c *captureReporter }

func (t *captureTask) Update(message string) { t.c.line(message) }
func (t *captureTask) Done(message string)   { t.c.line(message) }
func (t *captureTask) Fail(message string)   { t.c.line(message) }
