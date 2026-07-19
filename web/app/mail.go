package app

import (
	"context"
	"fmt"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/mail"
	"github.com/rs/zerolog/log"
)

// sendJobMail renders one job-notification template and sends it to the job's
// owner (the OIDC email is the address). It is a no-op when no mailer is
// configured, so scheduling and running work with SMTP disabled — only the
// notifications are missing.
func (a *App) sendJobMail(job *db.ScheduledJob, template, subject string) error {
	if a.mailer == nil {
		log.Debug().Str("job", job.ID).Msg("no mailer configured; skipping notification")
		return nil
	}
	data := mail.JobMail{
		Op:         job.Op,
		Course:     job.Course,
		Assignment: job.Assignment,
		RunAt:      job.RunAt,
		GraceMin:   job.GraceMin,
		Err:        job.Err,
		Log:        job.Log,
	}
	text, html, err := mail.Render(template, data)
	if err != nil {
		return err
	}
	return a.mailer.Send(a.mailDryRun, job.Owner, subject, text, html)
}

// terminalMailSpec maps a terminal status to its template and the subject word.
// Only done/failed/expired are mailed — a user-initiated cancel needs no mail
// (there is no cancellation template).
func terminalMailSpec(status string) (template, word string, ok bool) {
	switch status {
	case db.JobDone:
		return mail.TmplDone, "erfolgreich", true
	case db.JobFailed:
		return mail.TmplFailed, "fehlgeschlagen", true
	case db.JobExpired:
		return mail.TmplExpired, "abgelaufen", true
	}
	return "", "", false
}

// notifyFinishedJobs mails the outcome of every finished-but-unnotified job, then
// flags it notified so it is not mailed again. A send failure leaves the job
// unnotified, so the next poll tick retries it — the poll interval is the backoff.
func (a *App) notifyFinishedJobs(ctx context.Context) {
	if a.mailer == nil {
		return
	}
	jobs, err := a.db.UnnotifiedTerminalJobs(ctx)
	if err != nil {
		log.Error().Err(err).Msg("cannot read unnotified jobs")
		return
	}
	for _, job := range jobs {
		template, word, ok := terminalMailSpec(job.Status)
		if !ok {
			continue
		}
		subject := fmt.Sprintf("glabs: %s für %s/%s %s", job.Op, job.Course, job.Assignment, word)
		if err := a.sendJobMail(job, template, subject); err != nil {
			log.Warn().Err(err).Str("job", job.ID).Msg("cannot send job notification; will retry next tick")
			continue
		}
		if err := a.db.MarkNotified(ctx, job.ID); err != nil {
			log.Error().Err(err).Str("job", job.ID).Msg("notification sent but cannot mark job notified")
		}
	}
}

// sendScheduledConfirmation sends the "operation scheduled" confirmation for a
// freshly created job. Best-effort: a mail failure must not fail the scheduling.
func (a *App) sendScheduledConfirmation(job *db.ScheduledJob) {
	subject := fmt.Sprintf("glabs: %s für %s/%s geplant", job.Op, job.Course, job.Assignment)
	if err := a.sendJobMail(job, mail.TmplScheduled, subject); err != nil {
		log.Warn().Err(err).Str("job", job.ID).Msg("cannot send scheduling confirmation")
	}
}
