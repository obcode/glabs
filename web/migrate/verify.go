package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/google/go-cmp/cmp"
	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/secrets"
)

// verify compares the two databases and reports what it found.
//
// Three checks, in increasing strength:
//
//  1. Row counts per table. Cheap, and catches a collection that was skipped
//     entirely.
//  2. A full read-back of every record through the store's own API, diffed
//     field by field against what MongoDB held. At this data volume everything
//     fits in memory, which makes the strongest possible check also the cheapest
//     one -- it is the check that would notice a json tag that does not
//     round-trip, a timestamp that lost its zone, or a nil that became empty.
//  3. A cryptographic spot-check over every stored token: decrypt both sides and
//     compare a hash of the plaintexts. This is the one check that actually
//     proves nobody has to type their GitLab token back in.
//
// The plaintext is never printed, never logged and never compared as a string --
// only the SHA-256 of it, and only against the other side's.
func verify(ctx context.Context, out io.Writer, pg *db.PG, src *source, sealer *secrets.Sealer) error {
	var problems []string
	note := func(format string, args ...any) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}

	// --- 1. counts ---------------------------------------------------------
	got, err := pg.CountsForImport(ctx)
	if err != nil {
		return err
	}
	want := src.Counts()
	printf(out, "\nverification:\n")
	printf(out, "  %-16s %8s %8s  %s\n", "table", "mongodb", "postgres", "")
	for _, table := range db.ImportTables {
		// system_state is excluded from the count, and only from the count: the
		// row is seeded by the schema migration, so PostgreSQL always has exactly
		// one whether or not MongoDB ever wrote a document. Its CONTENT is
		// compared below, which is the question that actually matters.
		if table == "system_state" {
			printf(out, "  %-16s %8s %8d  (seeded, content checked below)\n",
				table, "-", got[table])
			continue
		}
		ok := "ok"
		if got[table] != want[table] {
			ok = "MISMATCH"
			note("%s: mongodb has %d rows, postgresql %d", table, want[table], got[table])
		}
		printf(out, "  %-16s %8d %8d  %s\n", table, want[table], got[table], ok)
	}

	// --- 2. full read-back --------------------------------------------------
	owners := src.owners()
	sort.Strings(owners)

	var pgCourses []*db.StoredCourse
	var pgJobs []*db.ScheduledJob
	var pgActivity []*db.ActivityEntry
	for _, owner := range owners {
		cs, err := pg.CoursesOf(ctx, owner)
		if err != nil {
			return err
		}
		pgCourses = append(pgCourses, cs...)

		js, err := pg.JobsOf(ctx, owner, nil)
		if err != nil {
			return err
		}
		pgJobs = append(pgJobs, js...)

		as, err := pg.AllActivityFor(ctx, owner)
		if err != nil {
			return err
		}
		pgActivity = append(pgActivity, as...)
	}

	// Both sides sorted the same way before diffing. The two stores do not
	// promise the same order for records sharing a timestamp, and that
	// difference is not a migration error.
	sortCourses(src.Courses)
	sortCourses(pgCourses)
	if diff := cmp.Diff(src.Courses, pgCourses); diff != "" {
		note("courses differ (-mongodb +postgresql):\n%s", diff)
	}

	sortJobs(src.Jobs)
	sortJobs(pgJobs)
	if diff := cmp.Diff(src.Jobs, pgJobs); diff != "" {
		note("scheduled jobs differ (-mongodb +postgresql):\n%s", diff)
	}

	sortActivity(src.Activity)
	sortActivity(pgActivity)
	if diff := cmp.Diff(src.Activity, pgActivity); diff != "" {
		note("activity differs (-mongodb +postgresql):\n%s", diff)
	}

	if len(src.Events) > 0 {
		// Events are not owner-scoped; the admin feed reads them all.
		oldest := src.Events[0].At
		for _, e := range src.Events {
			if e.At.Before(oldest) {
				oldest = e.At
			}
		}
		pgEvents, err := pg.RecentEvents(ctx, oldest, 0)
		if err != nil {
			return err
		}
		sortEvents(src.Events)
		sortEvents(pgEvents)
		if diff := cmp.Diff(src.Events, pgEvents); diff != "" {
			note("events differ (-mongodb +postgresql):\n%s", diff)
		}
	}

	state, err := pg.SystemState(ctx)
	if err != nil {
		return err
	}
	switch {
	case src.State == nil || src.State.SummarySentAt == nil:
		if state.SummarySentAt != nil {
			note("system state: mongodb had no summary timestamp, postgresql has %v", state.SummarySentAt)
		}
	case state.SummarySentAt == nil:
		note("system state: the summary timestamp was not carried over")
	case !state.SummarySentAt.Equal(*src.State.SummarySentAt):
		note("system state: summary timestamp is %v, want %v", state.SummarySentAt, src.State.SummarySentAt)
	}

	// --- 3. tokens ----------------------------------------------------------
	checked := 0
	for _, s := range src.Secrets {
		if s.GitLab == nil {
			continue
		}
		stored, err := pg.GetUserSecret(ctx, s.Owner)
		if err != nil {
			return err
		}
		if stored == nil || stored.GitLab == nil {
			note("the GitLab token of %s is missing from postgresql", s.Owner)
			continue
		}
		if sealer == nil {
			continue
		}
		fromMongo, err := sealer.Open(*s.GitLab)
		if err != nil {
			note("the GitLab token of %s cannot be decrypted from MONGODB (%v) -- "+
				"it was already unusable before this migration", s.Owner, err)
			continue
		}
		fromPG, err := sealer.Open(*stored.GitLab)
		if err != nil {
			note("the GitLab token of %s cannot be decrypted from postgresql: %v", s.Owner, err)
			continue
		}
		if digest(fromMongo) != digest(fromPG) {
			note("the GitLab token of %s decrypts to something different in postgresql", s.Owner)
			continue
		}
		checked++
	}
	if sealer == nil {
		println(out, "  tokens           skipped (no secrets.key)")
	} else {
		printf(out, "  tokens           %d decrypted on both sides to the same value\n", checked)
	}

	if len(problems) > 0 {
		printf(out, "\n%d problem(s):\n", len(problems))
		for _, p := range problems {
			printf(out, "  - %s\n", p)
		}
		return errors.New("verification failed -- do NOT switch db.uri over")
	}

	println(out, "\nverification passed: both databases hold the same data.")
	return nil
}

// digest hashes a decrypted token so the comparison never holds two plaintexts
// next to each other in a comparable form, and so an accidental %v of the
// compared value cannot leak one.
func digest(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

func sortCourses(cs []*db.StoredCourse) {
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].Owner != cs[j].Owner {
			return cs[i].Owner < cs[j].Owner
		}
		return cs[i].Name < cs[j].Name
	})
}

func sortJobs(js []*db.ScheduledJob) {
	sort.Slice(js, func(i, j int) bool { return js[i].ID < js[j].ID })
}

func sortActivity(as []*db.ActivityEntry) {
	sort.Slice(as, func(i, j int) bool {
		if !as[i].At.Equal(as[j].At) {
			return as[i].At.Before(as[j].At)
		}
		if as[i].Owner != as[j].Owner {
			return as[i].Owner < as[j].Owner
		}
		if as[i].Course != as[j].Course {
			return as[i].Course < as[j].Course
		}
		if as[i].Assignment != as[j].Assignment {
			return as[i].Assignment < as[j].Assignment
		}
		return as[i].Op < as[j].Op
	})
}

func sortEvents(es []*db.Event) {
	sort.Slice(es, func(i, j int) bool {
		if !es[i].At.Equal(es[j].At) {
			return es[i].At.Before(es[j].At)
		}
		if es[i].Type != es[j].Type {
			return es[i].Type < es[j].Type
		}
		if es[i].Actor != es[j].Actor {
			return es[i].Actor < es[j].Actor
		}
		return es[i].Detail < es[j].Detail
	})
}
