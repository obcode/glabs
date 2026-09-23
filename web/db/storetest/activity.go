package storetest

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/obcode/glabs/v3/web/db"
)

// ActivityLimit is the cap the two GUI-facing activity reads apply. It mirrors
// the unexported activityLimit in web/db; the suite has to know the number to
// assert that the cap is there at all.
const ActivityLimit = 200

func runActivity(t *testing.T, newStore NewStore) {
	t.Helper()

	entry := func(owner, course, assignment, op string, at time.Time) *db.ActivityEntry {
		return &db.ActivityEntry{
			Owner: owner, Course: course, Assignment: assignment, Op: op,
			Params: map[string]string{"accesslevel": "developer"},
			Status: "done", Detail: "12 Repositories", At: at,
		}
	}

	t.Run("record and read back in full", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		want := entry("a@hm.edu", "fopra", "blatt01", "setaccess", SummerInstant)
		if err := s.RecordActivity(ctx, want); err != nil {
			t.Fatalf("RecordActivity: %v", err)
		}

		got, err := s.ActivityFor(ctx, "a@hm.edu", "fopra", "blatt01")
		if err != nil {
			t.Fatalf("ActivityFor: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d entries, want 1", len(got))
		}
		if diff := cmp.Diff(want, got[0]); diff != "" {
			t.Errorf("entry did not survive the round trip (-want +got):\n%s", diff)
		}
	})

	t.Run("absent params stay nil", func(t *testing.T) {
		// Most operations take no parameters, and `omitempty` means those entries
		// have no params field at all. A store that turns that into an empty map
		// changes `params == nil` under every caller — same len(), different
		// answer to "were there any".
		s := newStore(t)
		ctx := t.Context()

		e := entry("a@hm.edu", "fopra", "blatt01", "archive", SummerInstant)
		e.Params = nil
		if err := s.RecordActivity(ctx, e); err != nil {
			t.Fatalf("RecordActivity: %v", err)
		}

		got, err := s.ActivityFor(ctx, "a@hm.edu", "fopra", "blatt01")
		if err != nil {
			t.Fatalf("ActivityFor: %v", err)
		}
		if got[0].Params != nil {
			t.Errorf("Params = %v, want nil", got[0].Params)
		}
	})

	t.Run("the three reads widen from assignment to course to owner", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		base := SummerInstant
		for i, e := range []*db.ActivityEntry{
			entry("a@hm.edu", "fopra", "blatt01", "setaccess", base),
			entry("a@hm.edu", "fopra", "blatt02", "protect", base.Add(time.Minute)),
			entry("a@hm.edu", "sysprog", "blatt01", "archive", base.Add(2*time.Minute)),
			entry("b@hm.edu", "fopra", "blatt01", "delete", base.Add(3*time.Minute)),
		} {
			if err := s.RecordActivity(ctx, e); err != nil {
				t.Fatalf("RecordActivity %d: %v", i, err)
			}
		}

		perAssignment, err := s.ActivityFor(ctx, "a@hm.edu", "fopra", "blatt01")
		if err != nil {
			t.Fatalf("ActivityFor: %v", err)
		}
		if len(perAssignment) != 1 {
			t.Errorf("ActivityFor = %d entries, want 1", len(perAssignment))
		}

		perCourse, err := s.CourseActivityFor(ctx, "a@hm.edu", "fopra")
		if err != nil {
			t.Fatalf("CourseActivityFor: %v", err)
		}
		if len(perCourse) != 2 {
			t.Errorf("CourseActivityFor = %d entries, want 2", len(perCourse))
		}

		perOwner, err := s.AllActivityFor(ctx, "a@hm.edu")
		if err != nil {
			t.Fatalf("AllActivityFor: %v", err)
		}
		if len(perOwner) != 3 {
			t.Errorf("AllActivityFor = %d entries, want 3 — another owner's entry leaked in", len(perOwner))
		}
	})

	t.Run("entries come back newest first", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		base := SummerInstant
		for i, op := range []string{"aeltester", "mittlerer", "neuester"} {
			if err := s.RecordActivity(ctx,
				entry("a@hm.edu", "fopra", "blatt01", op, base.Add(time.Duration(i)*time.Minute))); err != nil {
				t.Fatalf("RecordActivity %s: %v", op, err)
			}
		}

		got, err := s.ActivityFor(ctx, "a@hm.edu", "fopra", "blatt01")
		if err != nil {
			t.Fatalf("ActivityFor: %v", err)
		}
		var ops []string
		for _, e := range got {
			ops = append(ops, e.Op)
		}
		if diff := cmp.Diff([]string{"neuester", "mittlerer", "aeltester"}, ops); diff != "" {
			t.Errorf("entries are not newest first (-want +got):\n%s", diff)
		}
	})

	t.Run("the GUI reads are capped and the dump is not", func(t *testing.T) {
		// The course page shows recent history, not an unbounded audit trail; the
		// export has to be complete. Both halves matter, so one fixture proves
		// both: write more than the cap, then read it each way.
		if testing.Short() {
			t.Skip("writes ActivityLimit+5 entries")
		}
		s := newStore(t)
		ctx := t.Context()

		const extra = 5
		base := SummerInstant
		for i := range ActivityLimit + extra {
			e := entry("a@hm.edu", "fopra", "blatt01", fmt.Sprintf("op-%03d", i),
				base.Add(time.Duration(i)*time.Second))
			if err := s.RecordActivity(ctx, e); err != nil {
				t.Fatalf("RecordActivity %d: %v", i, err)
			}
		}

		capped, err := s.ActivityFor(ctx, "a@hm.edu", "fopra", "blatt01")
		if err != nil {
			t.Fatalf("ActivityFor: %v", err)
		}
		if len(capped) != ActivityLimit {
			t.Errorf("ActivityFor = %d entries, want the cap of %d", len(capped), ActivityLimit)
		}
		// Capped at the newest end: dropping the newest instead of the oldest
		// would leave the course page showing stale state.
		if len(capped) > 0 && capped[0].Op != fmt.Sprintf("op-%03d", ActivityLimit+extra-1) {
			t.Errorf("newest entry = %q, want the last one written", capped[0].Op)
		}

		cappedCourse, err := s.CourseActivityFor(ctx, "a@hm.edu", "fopra")
		if err != nil {
			t.Fatalf("CourseActivityFor: %v", err)
		}
		if len(cappedCourse) != ActivityLimit {
			t.Errorf("CourseActivityFor = %d entries, want the cap of %d", len(cappedCourse), ActivityLimit)
		}

		full, err := s.AllActivityFor(ctx, "a@hm.edu")
		if err != nil {
			t.Fatalf("AllActivityFor: %v", err)
		}
		if len(full) != ActivityLimit+extra {
			t.Errorf("AllActivityFor = %d entries, want all %d — the dump must not be capped",
				len(full), ActivityLimit+extra)
		}
	})

	t.Run("one owner never sees another's log", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		if err := s.RecordActivity(ctx, entry("a@hm.edu", "fopra", "blatt01", "setaccess", SummerInstant)); err != nil {
			t.Fatalf("RecordActivity: %v", err)
		}

		for _, read := range []struct {
			name string
			fn   func() ([]*db.ActivityEntry, error)
		}{
			{"ActivityFor", func() ([]*db.ActivityEntry, error) {
				return s.ActivityFor(ctx, "b@hm.edu", "fopra", "blatt01")
			}},
			{"CourseActivityFor", func() ([]*db.ActivityEntry, error) {
				return s.CourseActivityFor(ctx, "b@hm.edu", "fopra")
			}},
			{"AllActivityFor", func() ([]*db.ActivityEntry, error) {
				return s.AllActivityFor(ctx, "b@hm.edu")
			}},
		} {
			got, err := read.fn()
			if err != nil {
				t.Errorf("%s: %v", read.name, err)
				continue
			}
			if len(got) != 0 {
				t.Errorf("%s as another owner = %d entries, want 0", read.name, len(got))
			}
		}
	})
}
