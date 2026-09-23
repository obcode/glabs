package storetest

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/obcode/glabs/v3/web/db"
)

func runJobs(t *testing.T, newStore NewStore) {
	t.Helper()

	job := func(owner string, runAt time.Time) *db.ScheduledJob {
		return &db.ScheduledJob{
			Owner: owner, Op: "setaccess", Course: "fopra", Assignment: "blatt01",
			OnlyFor:    []string{"a@hm.edu", "b@hm.edu"},
			Params:     map[string]string{"accesslevel": "developer"},
			RunAt:      runAt,
			ConfigHash: "sha256:cafe",
			Status:     db.JobPending,
			GraceMin:   30,
			CreatedAt:  SummerInstant(),
		}
	}

	t.Run("save and read back in full", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		want := job("a@hm.edu", SummerInstant())
		if err := s.SaveJob(ctx, want); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}
		if want.ID == "" {
			t.Fatal("SaveJob left ID empty — the caller reads it off the struct afterwards")
		}

		got, err := s.JobOf(ctx, "a@hm.edu", want.ID)
		if err != nil {
			t.Fatalf("JobOf: %v", err)
		}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("job did not survive the round trip (-want +got):\n%s", diff)
		}
	})

	t.Run("a supplied id is kept", func(t *testing.T) {
		s := newStore(t)
		j := job("a@hm.edu", SummerInstant())
		j.ID = "507f1f77bcf86cd799439011" // the 24-hex shape production is full of
		if err := s.SaveJob(t.Context(), j); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}
		if j.ID != "507f1f77bcf86cd799439011" {
			t.Errorf("ID = %q, want the one supplied", j.ID)
		}
		if _, err := s.JobOf(t.Context(), "a@hm.edu", "507f1f77bcf86cd799439011"); err != nil {
			t.Errorf("JobOf by the supplied id: %v", err)
		}
	})

	t.Run("ids are unique across saves", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		seen := map[string]bool{}
		for range 50 {
			j := job("a@hm.edu", SummerInstant())
			if err := s.SaveJob(ctx, j); err != nil {
				t.Fatalf("SaveJob: %v", err)
			}
			if seen[j.ID] {
				t.Fatalf("id %q handed out twice", j.ID)
			}
			seen[j.ID] = true
		}
	})

	t.Run("absent lists and maps stay nil", func(t *testing.T) {
		// OnlyFor nil means "the whole assignment", not "nobody". The difference
		// decides which repositories an operation touches, so it must not be
		// blurred into an empty slice by the storage layer.
		s := newStore(t)
		ctx := t.Context()

		j := job("a@hm.edu", SummerInstant())
		j.OnlyFor, j.Params = nil, nil
		if err := s.SaveJob(ctx, j); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}

		got, err := s.JobOf(ctx, "a@hm.edu", j.ID)
		if err != nil {
			t.Fatalf("JobOf: %v", err)
		}
		if got.OnlyFor != nil {
			t.Errorf("OnlyFor = %v, want nil", got.OnlyFor)
		}
		if got.Params != nil {
			t.Errorf("Params = %v, want nil", got.Params)
		}
	})

	t.Run("claiming takes the oldest due job and flips it to running", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		now := SummerInstant()
		older := job("a@hm.edu", now.Add(-2*time.Hour))
		newer := job("a@hm.edu", now.Add(-1*time.Hour))
		for _, j := range []*db.ScheduledJob{newer, older} { // saved out of order on purpose
			if err := s.SaveJob(ctx, j); err != nil {
				t.Fatalf("SaveJob: %v", err)
			}
		}

		claimed, err := s.ClaimDueJob(ctx, "worker-1", now)
		if err != nil {
			t.Fatalf("ClaimDueJob: %v", err)
		}
		if claimed.ID != older.ID {
			t.Errorf("claimed the job scheduled for %v, want the older one at %v", claimed.RunAt, older.RunAt)
		}
		if claimed.Status != db.JobRunning {
			t.Errorf("Status = %q, want %q", claimed.Status, db.JobRunning)
		}
		if claimed.WorkerID != "worker-1" {
			t.Errorf("WorkerID = %q, want worker-1", claimed.WorkerID)
		}
		if claimed.StartedAt == nil {
			t.Error("StartedAt = nil, want the claim time")
		}
		// The returned document is the one after the update, so the caller can
		// run the job straight from it.
		stored, err := s.JobOf(ctx, "a@hm.edu", older.ID)
		if err != nil {
			t.Fatalf("JobOf: %v", err)
		}
		if stored.Status != db.JobRunning {
			t.Errorf("stored Status = %q, want %q — the claim was not persisted", stored.Status, db.JobRunning)
		}
	})

	t.Run("a job whose time has not come is not claimed", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		now := SummerInstant()
		if err := s.SaveJob(ctx, job("a@hm.edu", now.Add(time.Hour))); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}
		if _, err := s.ClaimDueJob(ctx, "worker-1", now); !errors.Is(err, db.ErrNoDueJob) {
			t.Errorf("ClaimDueJob: err = %v, want ErrNoDueJob", err)
		}
	})

	t.Run("nothing to claim is ErrNoDueJob, not an empty job", func(t *testing.T) {
		s := newStore(t)
		got, err := s.ClaimDueJob(t.Context(), "worker-1", SummerInstant())
		if !errors.Is(err, db.ErrNoDueJob) {
			t.Errorf("err = %v, want ErrNoDueJob", err)
		}
		if got != nil {
			t.Errorf("job = %+v, want nil", got)
		}
	})

	t.Run("only pending jobs are claimable", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		now := SummerInstant()
		for _, status := range []string{db.JobRunning, db.JobDone, db.JobFailed, db.JobExpired, db.JobCancelled} {
			j := job("a@hm.edu", now.Add(-time.Hour))
			j.Status = status
			if err := s.SaveJob(ctx, j); err != nil {
				t.Fatalf("SaveJob %s: %v", status, err)
			}
		}
		if _, err := s.ClaimDueJob(ctx, "worker-1", now); !errors.Is(err, db.ErrNoDueJob) {
			t.Errorf("ClaimDueJob: err = %v, want ErrNoDueJob", err)
		}
	})

	t.Run("concurrent claims never hand out the same job twice", func(t *testing.T) {
		// This is the one test that actually exercises the atomicity the whole
		// scheduling design rests on: the claim is what replaces a distributed
		// lock, so a job handed to two runners would be executed twice — for
		// `delete` or `archive` that is not a retry, it is damage.
		s := newStore(t)
		ctx := t.Context()

		const (
			dueJobs = 3
			workers = 10
		)
		now := SummerInstant()
		for range dueJobs {
			if err := s.SaveJob(ctx, job("a@hm.edu", now.Add(-time.Hour))); err != nil {
				t.Fatalf("SaveJob: %v", err)
			}
		}

		var (
			mu      sync.Mutex
			claimed []string
			wg      sync.WaitGroup
		)
		start := make(chan struct{})
		for w := range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start // release them together, so the claims really do overlap
				// Bounded on purpose. A store that keeps handing out the same job
				// is exactly what this test is looking for, and an unbounded loop
				// would spin against it until the whole binary times out minutes
				// later with no useful output. One attempt more than there are
				// jobs is enough for a correct store to reach ErrNoDueJob.
				for range dueJobs + 1 {
					j, err := s.ClaimDueJob(ctx, "worker", now)
					if errors.Is(err, db.ErrNoDueJob) {
						return
					}
					if err != nil {
						t.Errorf("worker %d: ClaimDueJob: %v", w, err)
						return
					}
					mu.Lock()
					claimed = append(claimed, j.ID)
					mu.Unlock()
				}
			}()
		}
		close(start)
		wg.Wait()

		if len(claimed) != dueJobs {
			t.Errorf("%d claims for %d due jobs — a job was handed out more than once", len(claimed), dueJobs)
		}
		seen := map[string]bool{}
		for _, id := range claimed {
			if seen[id] {
				t.Errorf("job %s was claimed twice", id)
			}
			seen[id] = true
		}
	})

	t.Run("finishing records the outcome and the time", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		j := job("a@hm.edu", SummerInstant())
		if err := s.SaveJob(ctx, j); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}
		if err := s.FinishJob(ctx, j.ID, db.JobFailed, "ausgabe", "es ging schief"); err != nil {
			t.Fatalf("FinishJob: %v", err)
		}

		got, err := s.JobOf(ctx, "a@hm.edu", j.ID)
		if err != nil {
			t.Fatalf("JobOf: %v", err)
		}
		if got.Status != db.JobFailed {
			t.Errorf("Status = %q, want %q", got.Status, db.JobFailed)
		}
		if got.Log != "ausgabe" || got.Err != "es ging schief" {
			t.Errorf("Log/Err = %q/%q, want the ones passed in", got.Log, got.Err)
		}
		// FinishedAt is set by the store, not the caller — it is also what the
		// retention sweep keys on, so a job without it would never be reaped.
		if got.FinishedAt == nil {
			t.Error("FinishedAt = nil, want the time the job finished")
		}
	})

	t.Run("finishing or notifying an unknown job is a no-op", func(t *testing.T) {
		// Pinned as it is: both are fire-and-forget updates that match nothing
		// and report success. The runner treats an error here as worth logging,
		// so turning these into errors would produce noise on a job the
		// retention sweep has already removed.
		s := newStore(t)
		ctx := t.Context()

		if err := s.FinishJob(ctx, "gibtsnicht", db.JobDone, "", ""); err != nil {
			t.Errorf("FinishJob for an unknown id: err = %v, want nil", err)
		}
		if err := s.MarkNotified(ctx, "gibtsnicht"); err != nil {
			t.Errorf("MarkNotified for an unknown id: err = %v, want nil", err)
		}
	})

	t.Run("the notify sweep sees terminal jobs until they are marked", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		// One job per status, all belonging to different owners — the sweep is
		// cross-owner by design, it feeds one mail per job.
		ids := map[string]string{}
		for _, status := range []string{db.JobDone, db.JobFailed, db.JobExpired, db.JobCancelled, db.JobPending, db.JobRunning} {
			j := job("a@hm.edu", SummerInstant())
			j.Status = status
			if err := s.SaveJob(ctx, j); err != nil {
				t.Fatalf("SaveJob %s: %v", status, err)
			}
			ids[status] = j.ID
		}

		got, err := s.UnnotifiedTerminalJobs(ctx)
		if err != nil {
			t.Fatalf("UnnotifiedTerminalJobs: %v", err)
		}
		gotStatus := map[string]bool{}
		for _, j := range got {
			gotStatus[j.Status] = true
		}
		for _, want := range []string{db.JobDone, db.JobFailed, db.JobExpired} {
			if !gotStatus[want] {
				t.Errorf("%s job is missing from the notify sweep", want)
			}
		}
		// Cancelling is something the user did on purpose; there is no mail for it.
		if gotStatus[db.JobCancelled] {
			t.Error("cancelled job turned up in the notify sweep")
		}
		if gotStatus[db.JobPending] || gotStatus[db.JobRunning] {
			t.Error("a job that has not finished turned up in the notify sweep")
		}

		if err := s.MarkNotified(ctx, ids[db.JobDone]); err != nil {
			t.Fatalf("MarkNotified: %v", err)
		}
		after, err := s.UnnotifiedTerminalJobs(ctx)
		if err != nil {
			t.Fatalf("UnnotifiedTerminalJobs (after): %v", err)
		}
		for _, j := range after {
			if j.ID == ids[db.JobDone] {
				t.Error("a job still shows up after being marked notified — its mail would be sent again")
			}
		}
		if len(after) != len(got)-1 {
			t.Errorf("sweep returned %d jobs, want %d", len(after), len(got)-1)
		}
	})

	t.Run("cancelling is only possible for one's own pending job", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		pending := job("a@hm.edu", SummerInstant())
		running := job("a@hm.edu", SummerInstant())
		running.Status = db.JobRunning
		for _, j := range []*db.ScheduledJob{pending, running} {
			if err := s.SaveJob(ctx, j); err != nil {
				t.Fatalf("SaveJob: %v", err)
			}
		}

		cancelled, err := s.CancelJob(ctx, "a@hm.edu", pending.ID)
		if err != nil {
			t.Fatalf("CancelJob: %v", err)
		}
		if cancelled.Status != db.JobCancelled {
			t.Errorf("Status = %q, want %q", cancelled.Status, db.JobCancelled)
		}
		if cancelled.FinishedAt == nil {
			t.Error("FinishedAt = nil, want the cancellation time")
		}

		// Already running: too late. Another owner: invisible. Both report the
		// same error, deliberately — "not found" must not become an oracle for
		// whether someone else has a job with that id.
		if _, err := s.CancelJob(ctx, "a@hm.edu", running.ID); !errors.Is(err, db.ErrJobNotFound) {
			t.Errorf("CancelJob on a running job: err = %v, want ErrJobNotFound", err)
		}
		if _, err := s.CancelJob(ctx, "a@hm.edu", pending.ID); !errors.Is(err, db.ErrJobNotFound) {
			t.Errorf("CancelJob twice: err = %v, want ErrJobNotFound", err)
		}
	})

	t.Run("jobs belong to exactly one owner", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		j := job("a@hm.edu", SummerInstant())
		if err := s.SaveJob(ctx, j); err != nil {
			t.Fatalf("SaveJob: %v", err)
		}

		if _, err := s.JobOf(ctx, "b@hm.edu", j.ID); !errors.Is(err, db.ErrJobNotFound) {
			t.Errorf("JobOf as another owner: err = %v, want ErrJobNotFound", err)
		}
		if _, err := s.CancelJob(ctx, "b@hm.edu", j.ID); !errors.Is(err, db.ErrJobNotFound) {
			t.Errorf("CancelJob as another owner: err = %v, want ErrJobNotFound", err)
		}
		if got, err := s.JobsOf(ctx, "b@hm.edu", nil); err != nil || len(got) != 0 {
			t.Errorf("JobsOf as another owner = %d jobs, %v; want 0, nil", len(got), err)
		}
		// And it really was not cancelled along the way.
		if still, err := s.JobOf(ctx, "a@hm.edu", j.ID); err != nil || still.Status != db.JobPending {
			t.Errorf("the owner's job is %+v after the foreign cancel, want still pending", still)
		}
	})

	t.Run("listing is newest scheduled first and filters by status", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		base := SummerInstant()
		early := job("a@hm.edu", base)
		late := job("a@hm.edu", base.Add(time.Hour))
		late.Status = db.JobDone
		for _, j := range []*db.ScheduledJob{early, late} {
			if err := s.SaveJob(ctx, j); err != nil {
				t.Fatalf("SaveJob: %v", err)
			}
		}

		all, err := s.JobsOf(ctx, "a@hm.edu", nil)
		if err != nil {
			t.Fatalf("JobsOf: %v", err)
		}
		if len(all) != 2 {
			t.Fatalf("JobsOf = %d jobs, want 2", len(all))
		}
		if all[0].ID != late.ID {
			t.Error("jobs are not ordered by runAt descending")
		}

		// An empty status list means "no filter", not "match nothing" — the GUI
		// passes nil for the unfiltered view.
		if empty, err := s.JobsOf(ctx, "a@hm.edu", []string{}); err != nil || len(empty) != 2 {
			t.Errorf("JobsOf with an empty filter = %d jobs, %v; want 2, nil", len(empty), err)
		}

		pending, err := s.JobsOf(ctx, "a@hm.edu", []string{db.JobPending})
		if err != nil {
			t.Fatalf("JobsOf filtered: %v", err)
		}
		if len(pending) != 1 || pending[0].ID != early.ID {
			t.Errorf("filtering for pending returned %d jobs, want only the pending one", len(pending))
		}

		both, err := s.JobsOf(ctx, "a@hm.edu", []string{db.JobPending, db.JobDone})
		if err != nil {
			t.Fatalf("JobsOf filtered (two): %v", err)
		}
		if len(both) != 2 {
			t.Errorf("filtering for two statuses returned %d jobs, want 2", len(both))
		}
	})

	t.Run("reading a job that does not exist", func(t *testing.T) {
		s := newStore(t)
		if _, err := s.JobOf(t.Context(), "a@hm.edu", "gibtsnicht"); !errors.Is(err, db.ErrJobNotFound) {
			t.Errorf("JobOf: err = %v, want ErrJobNotFound", err)
		}
		if _, err := s.CancelJob(t.Context(), "a@hm.edu", "gibtsnicht"); !errors.Is(err, db.ErrJobNotFound) {
			t.Errorf("CancelJob: err = %v, want ErrJobNotFound", err)
		}
	})
}
