package storetest

import (
	"testing"
	"time"

	"github.com/obcode/glabs/v3/web/db"
)

// runTimezone pins that a timestamp comes back in the zone it went in.
//
// The server runs with time.Local set to Europe/Berlin (cmd/glabs-web/main.go),
// and everything a human reads — the digest mail, the scheduled-for line on the
// course page, the admin feed — formats timestamps in it. A store that hands
// back UTC does not lose any information, so nothing fails; it just shows every
// time one or two hours early to every user.
//
// Both halves of the year are checked, because that is the only way to tell "the
// right zone" from "a fixed offset that happens to be right today": Berlin is
// +02:00 in July and +01:00 in January.
func runTimezone(t *testing.T, newStore NewStore) {
	t.Helper()

	if time.Local.String() != "Europe/Berlin" {
		t.Fatalf("time.Local is %s, want Europe/Berlin — the test binary must pin it in TestMain, "+
			"or this suite asserts a configuration that never exists in production", time.Local)
	}

	instants := map[string]time.Time{
		"summer (+02:00)": SummerInstant(),
		"winter (+01:00)": WinterInstant(),
	}

	check := func(t *testing.T, field string, want, got time.Time) {
		t.Helper()
		if !got.Equal(want) {
			t.Errorf("%s = %v, want the same instant as %v", field, got, want)
		}
		// The instant being equal is not enough: UTC and Berlin are equal as
		// instants and differ in every rendering.
		if got.Format(time.RFC3339) != want.Format(time.RFC3339) {
			t.Errorf("%s came back as %s, want %s — the zone was lost on the way through the database",
				field, got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
		_, wantOffset := want.Zone()
		if _, gotOffset := got.Zone(); gotOffset != wantOffset {
			t.Errorf("%s has UTC offset %ds, want %ds", field, gotOffset, wantOffset)
		}
	}

	for name, instant := range instants {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			ctx := t.Context()

			if err := s.SaveCourse(ctx, &db.StoredCourse{
				Owner: "a@hm.edu", Name: "fopra", Source: FullCourseSource(),
				ImportedAt: instant, UpdatedAt: instant,
			}); err != nil {
				t.Fatalf("SaveCourse: %v", err)
			}
			course, err := s.CourseOf(ctx, "a@hm.edu", "fopra")
			if err != nil {
				t.Fatalf("CourseOf: %v", err)
			}
			check(t, "StoredCourse.ImportedAt", instant, course.ImportedAt)

			if err := s.SaveUserGitLabToken(ctx, "a@hm.edu", sealed(0x11), instant); err != nil {
				t.Fatalf("SaveUserGitLabToken: %v", err)
			}
			secret, err := s.GetUserSecret(ctx, "a@hm.edu")
			if err != nil {
				t.Fatalf("GetUserSecret: %v", err)
			}
			check(t, "UserSecret.GitLabUpdatedAt", instant, *secret.GitLabUpdatedAt)

			if err := s.RecordActivity(ctx, &db.ActivityEntry{
				Owner: "a@hm.edu", Course: "fopra", Assignment: "blatt01",
				Op: "setaccess", Status: "done", At: instant,
			}); err != nil {
				t.Fatalf("RecordActivity: %v", err)
			}
			entries, err := s.ActivityFor(ctx, "a@hm.edu", "fopra", "blatt01")
			if err != nil {
				t.Fatalf("ActivityFor: %v", err)
			}
			check(t, "ActivityEntry.At", instant, entries[0].At)

			j := &db.ScheduledJob{
				Owner: "a@hm.edu", Op: "setaccess", Course: "fopra", Assignment: "blatt01",
				RunAt: instant, Status: db.JobPending, CreatedAt: instant,
			}
			if err := s.SaveJob(ctx, j); err != nil {
				t.Fatalf("SaveJob: %v", err)
			}
			stored, err := s.JobOf(ctx, "a@hm.edu", j.ID)
			if err != nil {
				t.Fatalf("JobOf: %v", err)
			}
			check(t, "ScheduledJob.RunAt", instant, stored.RunAt)
			// A nullable timestamp takes a different path through both drivers,
			// so it gets its own check rather than being assumed to follow.
			if err := s.FinishJob(ctx, j.ID, db.JobDone, "", ""); err != nil {
				t.Fatalf("FinishJob: %v", err)
			}
			finished, err := s.JobOf(ctx, "a@hm.edu", j.ID)
			if err != nil {
				t.Fatalf("JobOf (finished): %v", err)
			}
			if finished.FinishedAt == nil {
				t.Fatal("FinishedAt = nil after FinishJob")
			}
			if loc := finished.FinishedAt.Location(); loc != time.Local {
				t.Errorf("ScheduledJob.FinishedAt is in %s, want %s", loc, time.Local)
			}

			if err := s.RecordEvent(ctx, &db.Event{
				At: instant, Type: db.EventLogin, Severity: db.SeverityInfo,
			}); err != nil {
				t.Fatalf("RecordEvent: %v", err)
			}
			events, err := s.RecentEvents(ctx, instant.Add(-time.Hour), 1)
			if err != nil {
				t.Fatalf("RecentEvents: %v", err)
			}
			check(t, "Event.At", instant, events[0].At)

			if err := s.SetSummarySentAt(ctx, instant); err != nil {
				t.Fatalf("SetSummarySentAt: %v", err)
			}
			state, err := s.SystemState(ctx)
			if err != nil {
				t.Fatalf("SystemState: %v", err)
			}
			check(t, "SystemState.SummarySentAt", instant, *state.SummarySentAt)
		})
	}
}
