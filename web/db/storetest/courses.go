package storetest

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/obcode/glabs/v3/web/db"
)

func runCourses(t *testing.T, newStore NewStore) {
	t.Helper()

	t.Run("save and read back in full", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		want := &db.StoredCourse{
			Owner:      "a@hm.edu",
			Name:       "fopra",
			Source:     FullCourseSource(),
			RawYAML:    []byte("fopra:\n  coursepath: fk07/fopra # comment kept verbatim\n"),
			ImportedAt: SummerInstant,
			UpdatedAt:  WinterInstant,
		}
		if err := s.SaveCourse(ctx, want); err != nil {
			t.Fatalf("SaveCourse: %v", err)
		}

		got, err := s.CourseOf(ctx, "a@hm.edu", "fopra")
		if err != nil {
			t.Fatalf("CourseOf: %v", err)
		}
		// The whole document, field by field. This is the assertion that a
		// storage format which silently drops a field has to fail.
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("course did not survive the round trip (-want +got):\n%s", diff)
		}
	})

	t.Run("raw yaml stays absent when it was absent", func(t *testing.T) {
		// A course created through the web has no uploaded file behind it. nil has
		// to come back as nil, not as an empty non-nil slice: the download path
		// asks `if len(RawYAML) > 0` today, but an empty file is not the same
		// answer as no file, and the difference is cheap to keep.
		s := newStore(t)
		ctx := t.Context()

		if err := s.SaveCourse(ctx, &db.StoredCourse{
			Owner: "a@hm.edu", Name: "ohne-yaml", Source: FullCourseSource(),
			ImportedAt: SummerInstant, UpdatedAt: SummerInstant,
		}); err != nil {
			t.Fatalf("SaveCourse: %v", err)
		}

		got, err := s.CourseOf(ctx, "a@hm.edu", "ohne-yaml")
		if err != nil {
			t.Fatalf("CourseOf: %v", err)
		}
		if got.RawYAML != nil {
			t.Errorf("RawYAML = %q, want nil", got.RawYAML)
		}
	})

	t.Run("save is an upsert on (owner, name)", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		first := &db.StoredCourse{
			Owner: "a@hm.edu", Name: "fopra", Source: FullCourseSource(),
			RawYAML: []byte("alt"), ImportedAt: SummerInstant, UpdatedAt: SummerInstant,
		}
		if err := s.SaveCourse(ctx, first); err != nil {
			t.Fatalf("SaveCourse: %v", err)
		}

		second := &db.StoredCourse{
			Owner: "a@hm.edu", Name: "fopra", Source: FullCourseSource(),
			RawYAML: []byte("neu"), ImportedAt: SummerInstant, UpdatedAt: WinterInstant,
		}
		if err := s.SaveCourse(ctx, second); err != nil {
			t.Fatalf("SaveCourse (second): %v", err)
		}

		all, err := s.CoursesOf(ctx, "a@hm.edu")
		if err != nil {
			t.Fatalf("CoursesOf: %v", err)
		}
		if len(all) != 1 {
			t.Fatalf("got %d courses, want 1 — the second save inserted instead of replacing", len(all))
		}
		if diff := cmp.Diff(second, all[0]); diff != "" {
			t.Errorf("stored course is not the second one (-want +got):\n%s", diff)
		}
	})

	t.Run("courses are listed by name within an owner", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		for _, name := range []string{"zzz", "aaa", "mmm"} {
			if err := s.SaveCourse(ctx, &db.StoredCourse{
				Owner: "a@hm.edu", Name: name, Source: FullCourseSource(),
				ImportedAt: SummerInstant, UpdatedAt: SummerInstant,
			}); err != nil {
				t.Fatalf("SaveCourse %s: %v", name, err)
			}
		}

		all, err := s.CoursesOf(ctx, "a@hm.edu")
		if err != nil {
			t.Fatalf("CoursesOf: %v", err)
		}
		var names []string
		for _, c := range all {
			names = append(names, c.Name)
		}
		if diff := cmp.Diff([]string{"aaa", "mmm", "zzz"}, names); diff != "" {
			t.Errorf("courses are not sorted by name (-want +got):\n%s", diff)
		}
	})

	t.Run("a course belongs to exactly one owner", func(t *testing.T) {
		// Owner isolation is the security property of this layer: to one user,
		// another user's course simply is not there. All four reads have to agree
		// on that, which is why they are asserted together.
		s := newStore(t)
		ctx := t.Context()

		if err := s.SaveCourse(ctx, &db.StoredCourse{
			Owner: "a@hm.edu", Name: "fopra", Source: FullCourseSource(),
			ImportedAt: SummerInstant, UpdatedAt: SummerInstant,
		}); err != nil {
			t.Fatalf("SaveCourse: %v", err)
		}

		if _, err := s.CourseOf(ctx, "b@hm.edu", "fopra"); !errors.Is(err, db.ErrCourseNotFound) {
			t.Errorf("CourseOf as another owner: err = %v, want ErrCourseNotFound", err)
		}
		if all, err := s.CoursesOf(ctx, "b@hm.edu"); err != nil || len(all) != 0 {
			t.Errorf("CoursesOf as another owner = %d courses, %v; want 0, nil", len(all), err)
		}
		// Not a silent success: a delete that reports nil would tell the other
		// user their delete worked, and the GUI would remove the row.
		if err := s.DeleteCourse(ctx, "b@hm.edu", "fopra"); !errors.Is(err, db.ErrCourseNotFound) {
			t.Errorf("DeleteCourse as another owner: err = %v, want ErrCourseNotFound", err)
		}
		if _, err := s.CourseOf(ctx, "a@hm.edu", "fopra"); err != nil {
			t.Errorf("the owner's own course is gone after the foreign delete: %v", err)
		}
	})

	t.Run("reading or deleting a course that does not exist", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		if _, err := s.CourseOf(ctx, "a@hm.edu", "gibtsnicht"); !errors.Is(err, db.ErrCourseNotFound) {
			t.Errorf("CourseOf: err = %v, want ErrCourseNotFound", err)
		}
		if err := s.DeleteCourse(ctx, "a@hm.edu", "gibtsnicht"); !errors.Is(err, db.ErrCourseNotFound) {
			t.Errorf("DeleteCourse: err = %v, want ErrCourseNotFound", err)
		}
	})

	t.Run("delete removes only the named course", func(t *testing.T) {
		s := newStore(t)
		ctx := t.Context()

		for _, name := range []string{"fopra", "sysprog"} {
			if err := s.SaveCourse(ctx, &db.StoredCourse{
				Owner: "a@hm.edu", Name: name, Source: FullCourseSource(),
				ImportedAt: SummerInstant, UpdatedAt: SummerInstant,
			}); err != nil {
				t.Fatalf("SaveCourse %s: %v", name, err)
			}
		}
		if err := s.DeleteCourse(ctx, "a@hm.edu", "fopra"); err != nil {
			t.Fatalf("DeleteCourse: %v", err)
		}

		all, err := s.CoursesOf(ctx, "a@hm.edu")
		if err != nil {
			t.Fatalf("CoursesOf: %v", err)
		}
		if len(all) != 1 || all[0].Name != "sysprog" {
			t.Errorf("after deleting fopra: %d courses left, want only sysprog", len(all))
		}
	})

	t.Run("a course without an owner is refused", func(t *testing.T) {
		// SaveCourse guards this explicitly, because an ownerless document would
		// be invisible to every owner-scoped read and so impossible to clean up
		// through the application.
		s := newStore(t)
		err := s.SaveCourse(t.Context(), &db.StoredCourse{
			Name: "fopra", Source: FullCourseSource(),
			ImportedAt: time.Now(), UpdatedAt: time.Now(),
		})
		if err == nil {
			t.Error("SaveCourse without an owner returned nil, want an error")
		}
	})
}
