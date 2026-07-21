package app

import (
	"context"
	"sort"
	"time"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/mail"
	"github.com/rs/zerolog/log"
)

// Summary is the aggregated, human-readable digest of a period's events — the
// shape both the nightly mail template and the admin page's summary render. It is
// deliberately pre-digested (counts and short lists), never raw log lines, which
// is exactly what the operator asked for.
type Summary struct {
	From        time.Time
	Until       time.Time
	TotalEvents int
	Quiet       bool // nothing happened in the window

	ActiveUsers    []UserActivity  // who was active, busiest first
	RejectedLogins []RejectedLogin // refused sign-ins, most attempts first (security)

	ScheduledJobs []EventLine // operations queued during the window
	JobDone       int
	JobFailed     int
	JobExpired    int
	JobCancelled  int
	JobsRun       int         // done+failed+expired+cancelled
	JobFailures   []EventLine // failed/expired runs, with the error

	OpDone     int
	OpFailed   int
	OpsByType  []LabelCount // interactive ops grouped by kind
	OpFailures []EventLine

	CourseCreated int
	CourseDeleted int
	TokenSaved    int
	TokenDeleted  int

	Problems []EventLine // everything at warning/error severity, newest first
}

// EventLine is one event projected for display in a digest section.
type EventLine struct {
	At         time.Time
	Type       string
	Severity   string
	Actor      string
	Course     string
	Assignment string
	Op         string
	Detail     string
}

// UserActivity is one active user with how often they were seen in the window.
type UserActivity struct {
	Email      string
	Name       string
	Department string
	Logins     int
}

// RejectedLogin is one refused identity with its attempt count.
type RejectedLogin struct {
	Email      string
	Department string
	Count      int
}

// LabelCount is a labelled tally (interactive ops by kind).
type LabelCount struct {
	Label string
	Count int
}

func toLine(e *db.Event) EventLine {
	return EventLine{
		At: e.At, Type: e.Type, Severity: e.Severity, Actor: e.Actor,
		Course: e.Course, Assignment: e.Assignment, Op: e.Op, Detail: e.Detail,
	}
}

// buildSummary aggregates a window's events into the digest. It is a pure function
// (no clock, no I/O) so it can be unit-tested against a fixed slice.
func buildSummary(from, until time.Time, events []*db.Event) *Summary {
	s := &Summary{From: from, Until: until, TotalEvents: len(events), Quiet: len(events) == 0}

	users := map[string]*UserActivity{}
	rejected := map[string]*RejectedLogin{}
	opsByType := map[string]int{}

	for _, e := range events {
		switch e.Type {
		case db.EventLogin:
			u := users[e.Actor]
			if u == nil {
				u = &UserActivity{Email: e.Actor, Name: e.ActorName, Department: e.Department}
				users[e.Actor] = u
			}
			u.Logins++
			if u.Name == "" {
				u.Name = e.ActorName
			}
			if u.Department == "" {
				u.Department = e.Department
			}
		case db.EventLoginRejected:
			key := e.Actor
			if key == "" {
				key = "(anonym)"
			}
			r := rejected[key]
			if r == nil {
				r = &RejectedLogin{Email: key, Department: e.Department}
				rejected[key] = r
			}
			r.Count++
		case db.EventJobScheduled:
			s.ScheduledJobs = append(s.ScheduledJobs, toLine(e))
		case db.EventJobDone:
			s.JobDone++
		case db.EventJobFailed:
			s.JobFailed++
			s.JobFailures = append(s.JobFailures, toLine(e))
		case db.EventJobExpired:
			s.JobExpired++
			s.JobFailures = append(s.JobFailures, toLine(e))
		case db.EventJobCancelled:
			s.JobCancelled++
		case db.EventOpDone:
			s.OpDone++
			opsByType[e.Op]++
		case db.EventOpFailed:
			s.OpFailed++
			opsByType[e.Op]++
			s.OpFailures = append(s.OpFailures, toLine(e))
		case db.EventCourseCreated:
			s.CourseCreated++
		case db.EventCourseDeleted:
			s.CourseDeleted++
		case db.EventTokenSaved:
			s.TokenSaved++
		case db.EventTokenDeleted:
			s.TokenDeleted++
		}
		if e.Severity == db.SeverityWarning || e.Severity == db.SeverityError {
			s.Problems = append(s.Problems, toLine(e))
		}
	}

	for _, u := range users {
		s.ActiveUsers = append(s.ActiveUsers, *u)
	}
	sort.Slice(s.ActiveUsers, func(i, j int) bool {
		if s.ActiveUsers[i].Logins != s.ActiveUsers[j].Logins {
			return s.ActiveUsers[i].Logins > s.ActiveUsers[j].Logins
		}
		return s.ActiveUsers[i].Email < s.ActiveUsers[j].Email
	})
	for _, r := range rejected {
		s.RejectedLogins = append(s.RejectedLogins, *r)
	}
	sort.Slice(s.RejectedLogins, func(i, j int) bool {
		if s.RejectedLogins[i].Count != s.RejectedLogins[j].Count {
			return s.RejectedLogins[i].Count > s.RejectedLogins[j].Count
		}
		return s.RejectedLogins[i].Email < s.RejectedLogins[j].Email
	})
	for op, n := range opsByType {
		s.OpsByType = append(s.OpsByType, LabelCount{Label: op, Count: n})
	}
	sort.Slice(s.OpsByType, func(i, j int) bool { return s.OpsByType[i].Label < s.OpsByType[j].Label })
	// Problems newest first, so the most recent issue leads.
	sort.SliceStable(s.Problems, func(i, j int) bool { return s.Problems[i].At.After(s.Problems[j].At) })

	s.JobsRun = s.JobDone + s.JobFailed + s.JobExpired + s.JobCancelled
	return s
}

// PlatformSummary aggregates the events in [since, until) into the digest — the
// same view the nightly mail sends, exposed so the admin page can show it live.
// Admin-only.
func (a *App) PlatformSummary(ctx context.Context, since, until time.Time) (*Summary, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	events, err := a.db.EventsBetween(ctx, since, until)
	if err != nil {
		return nil, err
	}
	return buildSummary(since, until, events), nil
}

// ConfigureSummary stores the nightly-summary recipients and hour so both the
// scheduler and the on-demand "send now" path use the same target.
func (a *App) ConfigureSummary(hour int, recipients []string) {
	if hour < 0 || hour > 23 {
		hour = 5
	}
	a.summaryHour = hour
	a.summaryRecipients = recipients
}

// StartSummaryMailer sends the nightly digest at the configured hour (local time)
// until ctx is cancelled. It is a no-op without a mailer or recipients. The window
// is (last-sent, now], persisted in Mongo, so a restart never double-sends and a
// missed night is folded into the next run.
func (a *App) StartSummaryMailer(ctx context.Context) {
	if a.mailer == nil || len(a.summaryRecipients) == 0 {
		log.Warn().Msg("nightly summary disabled (no mailer or no recipients configured)")
		return
	}
	log.Info().Int("hour", a.summaryHour).Strs("to", a.summaryRecipients).Msg("nightly summary mailer started")
	for {
		next := nextRun(time.Now(), a.summaryHour)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Info().Msg("nightly summary mailer stopped")
			return
		case <-timer.C:
		}
		a.runNightlySummary(ctx)
	}
}

// nextRun returns the next occurrence of hour:00 local time strictly after now.
func nextRun(now time.Time, hour int) time.Time {
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

// runNightlySummary sends the digest for everything since the last summary and, on
// success, advances the marker so the next run starts where this one ended.
func (a *App) runNightlySummary(ctx context.Context) {
	now := time.Now()
	since := now.Add(-24 * time.Hour)
	if state, err := a.db.SystemState(ctx); err != nil {
		log.Warn().Err(err).Msg("cannot read last-summary time; defaulting to the last 24h")
	} else if state.SummarySentAt != nil {
		since = *state.SummarySentAt
	}
	if err := a.sendSummary(ctx, a.summaryRecipients, since, now); err != nil {
		log.Warn().Err(err).Msg("cannot send nightly summary; will retry next night")
		return
	}
	if err := a.db.SetSummarySentAt(ctx, now); err != nil {
		log.Error().Err(err).Msg("summary sent but cannot record the sent time (risks a double-send)")
	}
}

// SendSummaryNow sends the digest for the last 24h on demand (the admin page's
// "send now" button), without moving the nightly marker — it is a preview, not the
// scheduled run. Admin-only.
func (a *App) SendSummaryNow(ctx context.Context) error {
	if err := a.requireAdmin(ctx); err != nil {
		return err
	}
	now := time.Now()
	return a.sendSummary(ctx, a.summaryRecipients, now.Add(-24*time.Hour), now)
}

// sendSummary builds and mails the digest for [since, until) to every recipient.
func (a *App) sendSummary(ctx context.Context, recipients []string, since, until time.Time) error {
	if a.mailer == nil {
		return nil
	}
	events, err := a.db.EventsBetween(ctx, since, until)
	if err != nil {
		return err
	}
	summary := buildSummary(since, until, events)
	text, html, err := mail.Render(mail.TmplAdminSummary, summary)
	if err != nil {
		return err
	}
	subject := "glabs: Nächtliche Zusammenfassung " + until.In(time.Local).Format("02.01.2006")
	for _, to := range recipients {
		if err := a.mailer.Send(a.mailDryRun, to, subject, text, html); err != nil {
			return err
		}
	}
	return nil
}
