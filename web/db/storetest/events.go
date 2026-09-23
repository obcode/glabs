package storetest

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/obcode/glabs/v3/web/db"
)

func runEvents(t *testing.T, newStore NewStore) {
	t.Helper()

	t.Run("record and read back in full", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		want := &db.Event{
			At: SummerInstant(), Type: db.EventJobFailed,
			Actor: "a@hm.edu", ActorName: "A. Beispiel", Department: "FK07",
			Course: "fopra", Assignment: "blatt01", Op: "setaccess",
			Severity: db.SeverityError, Detail: "es ging schief", JobID: "507f1f77bcf86cd799439011",
		}
		if err := s.RecordEvent(ctx, want); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}

		got, err := s.RecentEvents(ctx, SummerInstant().Add(-time.Hour), 10)
		if err != nil {
			t.Fatalf("RecentEvents: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d events, want 1", len(got))
		}
		if diff := cmp.Diff(want, got[0]); diff != "" {
			t.Errorf("event did not survive the round trip (-want +got):\n%s", diff)
		}
	})

	t.Run("a missing severity becomes info", func(t *testing.T) {
		// The default is applied by the store rather than by each caller, so a
		// forgotten field never hides an event from the admin page's filter.
		// Note it also writes back onto the caller's struct.
		s := newStore(t)
		ctx := t.Context()

		e := &db.Event{At: SummerInstant(), Type: db.EventLogin, Actor: "a@hm.edu"}
		if err := s.RecordEvent(ctx, e); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
		if e.Severity != db.SeverityInfo {
			t.Errorf("the caller's struct has Severity %q, want %q", e.Severity, db.SeverityInfo)
		}

		got, err := s.RecentEvents(ctx, SummerInstant().Add(-time.Hour), 10)
		if err != nil {
			t.Fatalf("RecentEvents: %v", err)
		}
		if len(got) != 1 || got[0].Severity != db.SeverityInfo {
			t.Errorf("stored severity = %q, want %q", got[0].Severity, db.SeverityInfo)
		}
	})

	t.Run("the digest window is half open and oldest first", func(t *testing.T) {
		// [since, until): the upper bound belongs to the NEXT window. Including
		// it would mail the same event in two consecutive digests.
		s := newStore(t)
		ctx := t.Context()

		base := SummerInstant()
		for _, at := range []time.Time{
			base.Add(-time.Second),  // before the window
			base,                    // the lower bound is included
			base.Add(time.Minute),   // inside
			base.Add(time.Hour),     // the upper bound is excluded
			base.Add(2 * time.Hour), // after
		} {
			if err := s.RecordEvent(ctx, &db.Event{At: at, Type: db.EventLogin, Severity: db.SeverityInfo}); err != nil {
				t.Fatalf("RecordEvent %v: %v", at, err)
			}
		}

		got, err := s.EventsBetween(ctx, base, base.Add(time.Hour))
		if err != nil {
			t.Fatalf("EventsBetween: %v", err)
		}
		var ats []time.Time
		for _, e := range got {
			ats = append(ats, e.At)
		}
		want := []time.Time{base, base.Add(time.Minute)}
		if len(ats) != len(want) {
			t.Fatalf("EventsBetween = %d events (%v), want %d", len(ats), ats, len(want))
		}
		for i := range want {
			if !ats[i].Equal(want[i]) {
				t.Errorf("event %d at %v, want %v — window bounds or ordering are wrong", i, ats[i], want[i])
			}
		}
	})

	t.Run("the admin feed is newest first and capped", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		base := SummerInstant()
		for i := range 5 {
			if err := s.RecordEvent(ctx, &db.Event{
				At: base.Add(time.Duration(i) * time.Minute), Type: db.EventLogin,
				Detail: string(rune('a' + i)), Severity: db.SeverityInfo,
			}); err != nil {
				t.Fatalf("RecordEvent %d: %v", i, err)
			}
		}

		got, err := s.RecentEvents(ctx, base, 3)
		if err != nil {
			t.Fatalf("RecentEvents: %v", err)
		}
		if len(got) != 3 {
			t.Fatalf("RecentEvents = %d events, want the limit of 3", len(got))
		}
		var details []string
		for _, e := range got {
			details = append(details, e.Detail)
		}
		// Newest first, and the cap drops the OLDEST — a feed that dropped the
		// newest would never show what just happened.
		if diff := cmp.Diff([]string{"e", "d", "c"}, details); diff != "" {
			t.Errorf("feed is not the newest three, newest first (-want +got):\n%s", diff)
		}

		// A limit of zero means no limit, matching the internal findEvents.
		all, err := s.RecentEvents(ctx, base, 0)
		if err != nil {
			t.Fatalf("RecentEvents unlimited: %v", err)
		}
		if len(all) != 5 {
			t.Errorf("RecentEvents with limit 0 = %d events, want all 5", len(all))
		}
	})

	t.Run("the event log is not owner scoped", func(t *testing.T) {
		// Deliberately the opposite rule from courses, activity and jobs: this
		// collection exists to be read across all users by an admin. Asserting it
		// keeps a well-meant "add an owner filter" from passing review.
		s := newStore(t)
		ctx := t.Context()

		for _, actor := range []string{"a@hm.edu", "b@hm.edu", ""} {
			if err := s.RecordEvent(ctx, &db.Event{
				At: SummerInstant(), Type: db.EventLogin, Actor: actor, Severity: db.SeverityInfo,
			}); err != nil {
				t.Fatalf("RecordEvent %q: %v", actor, err)
			}
		}

		got, err := s.RecentEvents(ctx, SummerInstant().Add(-time.Hour), 0)
		if err != nil {
			t.Fatalf("RecentEvents: %v", err)
		}
		if len(got) != 3 {
			t.Errorf("RecentEvents = %d events, want all 3 regardless of actor", len(got))
		}
	})
}
