package app

import (
	"bytes"
	"testing"
	"time"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/mail"
)

func TestBuildSummaryAggregates(t *testing.T) {
	base := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	at := func(min int) time.Time { return base.Add(time.Duration(min) * time.Minute) }

	events := []*db.Event{
		{At: at(1), Type: db.EventLogin, Actor: "prof@hm.edu", ActorName: "Prof", Department: "07", Severity: db.SeverityInfo},
		{At: at(2), Type: db.EventLogin, Actor: "prof@hm.edu", Severity: db.SeverityInfo},
		{At: at(3), Type: db.EventLogin, Actor: "tutor@hm.edu", Severity: db.SeverityInfo},
		{At: at(4), Type: db.EventLoginRejected, Actor: "stranger@hm.edu", Severity: db.SeverityWarning, Detail: "nicht auf der Allowlist"},
		{At: at(5), Type: db.EventLoginRejected, Actor: "stranger@hm.edu", Severity: db.SeverityWarning},
		{At: at(6), Type: db.EventJobScheduled, Actor: "prof@hm.edu", Op: "setaccess", Course: "tc", Assignment: "b1", Severity: db.SeverityInfo},
		{At: at(7), Type: db.EventJobDone, Actor: "prof@hm.edu", Op: "setaccess", Course: "tc", Assignment: "b1", Severity: db.SeverityInfo},
		{At: at(8), Type: db.EventJobFailed, Actor: "prof@hm.edu", Op: "delete", Course: "tc", Assignment: "b2", Severity: db.SeverityError, Detail: "boom"},
		{At: at(9), Type: db.EventOpDone, Actor: "prof@hm.edu", Op: "protect", Course: "tc", Assignment: "b1", Severity: db.SeverityInfo},
		{At: at(10), Type: db.EventOpFailed, Actor: "prof@hm.edu", Op: "protect", Course: "tc", Assignment: "b3", Severity: db.SeverityError, Detail: "nope"},
		{At: at(11), Type: db.EventCourseCreated, Actor: "prof@hm.edu", Course: "tc", Severity: db.SeverityInfo},
		{At: at(12), Type: db.EventTokenSaved, Actor: "prof@hm.edu", Severity: db.SeverityInfo},
	}

	s := buildSummary(base, at(60), events)

	if s.TotalEvents != len(events) {
		t.Errorf("TotalEvents = %d, want %d", s.TotalEvents, len(events))
	}
	if s.Quiet {
		t.Error("Quiet = true, want false")
	}
	// Two active users, prof busiest first with 2 logins, department carried over.
	if len(s.ActiveUsers) != 2 {
		t.Fatalf("ActiveUsers = %d, want 2", len(s.ActiveUsers))
	}
	if s.ActiveUsers[0].Email != "prof@hm.edu" || s.ActiveUsers[0].Logins != 2 {
		t.Errorf("busiest user = %+v, want prof with 2 logins", s.ActiveUsers[0])
	}
	if s.ActiveUsers[0].Department != "07" {
		t.Errorf("department not carried over: %q", s.ActiveUsers[0].Department)
	}
	// Rejected logins collapsed by email with a count.
	if len(s.RejectedLogins) != 1 || s.RejectedLogins[0].Count != 2 {
		t.Errorf("RejectedLogins = %+v, want one entry with count 2", s.RejectedLogins)
	}
	if len(s.ScheduledJobs) != 1 {
		t.Errorf("ScheduledJobs = %d, want 1", len(s.ScheduledJobs))
	}
	if s.JobDone != 1 || s.JobFailed != 1 || s.JobsRun != 2 {
		t.Errorf("jobs done=%d failed=%d run=%d, want 1/1/2", s.JobDone, s.JobFailed, s.JobsRun)
	}
	if len(s.JobFailures) != 1 || s.JobFailures[0].Detail != "boom" {
		t.Errorf("JobFailures = %+v, want one with detail boom", s.JobFailures)
	}
	if s.OpDone != 1 || s.OpFailed != 1 {
		t.Errorf("ops done=%d failed=%d, want 1/1", s.OpDone, s.OpFailed)
	}
	if len(s.OpsByType) != 1 || s.OpsByType[0].Label != "protect" || s.OpsByType[0].Count != 2 {
		t.Errorf("OpsByType = %+v, want protect:2", s.OpsByType)
	}
	if s.CourseCreated != 1 || s.TokenSaved != 1 {
		t.Errorf("courseCreated=%d tokenSaved=%d, want 1/1", s.CourseCreated, s.TokenSaved)
	}
	// Problems = the two warnings + two errors, newest first.
	if len(s.Problems) != 4 {
		t.Fatalf("Problems = %d, want 4", len(s.Problems))
	}
	if s.Problems[0].At.Before(s.Problems[1].At) {
		t.Error("Problems not sorted newest first")
	}
}

func TestBuildSummaryQuiet(t *testing.T) {
	s := buildSummary(time.Now().Add(-time.Hour), time.Now(), nil)
	if !s.Quiet || s.TotalEvents != 0 {
		t.Errorf("empty window: Quiet=%v total=%d, want true/0", s.Quiet, s.TotalEvents)
	}
}

// The summary template must render (both text and HTML) for a fully-populated and
// an empty digest — a template typo only surfaces at render time.
func TestSummaryTemplateRenders(t *testing.T) {
	base := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	events := []*db.Event{
		{At: base, Type: db.EventLogin, Actor: "prof@hm.edu", ActorName: "Prof", Department: "07", Severity: db.SeverityInfo},
		{At: base, Type: db.EventJobScheduled, Actor: "prof@hm.edu", Op: "setaccess", Course: "tc", Assignment: "b1", Severity: db.SeverityInfo},
		{At: base, Type: db.EventJobFailed, Actor: "prof@hm.edu", Op: "delete", Course: "tc", Assignment: "b2", Severity: db.SeverityError, Detail: "boom"},
		{At: base, Type: db.EventOpFailed, Actor: "prof@hm.edu", Op: "protect", Course: "tc", Assignment: "b3", Severity: db.SeverityError, Detail: "nope"},
		{At: base, Type: db.EventLoginRejected, Actor: "x@hm.edu", Severity: db.SeverityWarning},
		{At: base, Type: db.EventCourseCreated, Actor: "prof@hm.edu", Course: "tc", Severity: db.SeverityInfo},
	}
	for _, s := range []*Summary{buildSummary(base, base.Add(time.Hour), events), buildSummary(base, base.Add(time.Hour), nil)} {
		text, html, err := mail.Render(mail.TmplAdminSummary, s)
		if err != nil {
			t.Fatalf("render: %v", err)
		}
		if len(bytes.TrimSpace(text)) == 0 || len(bytes.TrimSpace(html)) == 0 {
			t.Error("rendered summary is empty")
		}
	}
}

func TestNextRun(t *testing.T) {
	loc := time.UTC
	// Before the hour today → today at that hour.
	now := time.Date(2026, 7, 21, 3, 30, 0, 0, loc)
	next := nextRun(now, 5)
	if next.Hour() != 5 || next.Day() != 21 {
		t.Errorf("nextRun before hour = %s, want today 05:00", next)
	}
	// After the hour → tomorrow.
	now = time.Date(2026, 7, 21, 6, 0, 0, 0, loc)
	next = nextRun(now, 5)
	if next.Hour() != 5 || next.Day() != 22 {
		t.Errorf("nextRun after hour = %s, want tomorrow 05:00", next)
	}
	// Exactly at the hour → tomorrow (strictly after now).
	now = time.Date(2026, 7, 21, 5, 0, 0, 0, loc)
	if got := nextRun(now, 5); got.Day() != 22 {
		t.Errorf("nextRun at hour = %s, want tomorrow", got)
	}
}
