package app

import (
	"context"
	"testing"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/graph/model"
	"github.com/obcode/glabs/v3/web/principal"
)

// fakeStore records the owner each method was called with, so a test can assert
// the app always scopes to the authenticated user.
type fakeStore struct {
	seenOwners []string
	courses    map[string]*db.StoredCourse // keyed by owner+"/"+name
	saved      *db.StoredCourse
}

func newFakeStore() *fakeStore {
	return &fakeStore{courses: map[string]*db.StoredCourse{}}
}

func (f *fakeStore) GetUserByEmail(context.Context, string) (*model.User, error) { return nil, nil }

func (f *fakeStore) CoursesOf(_ context.Context, owner string) ([]*db.StoredCourse, error) {
	f.seenOwners = append(f.seenOwners, owner)
	var out []*db.StoredCourse
	for _, c := range f.courses {
		if c.Owner == owner {
			out = append(out, c)
		}
	}
	return out, nil
}

func (f *fakeStore) CourseOf(_ context.Context, owner, name string) (*db.StoredCourse, error) {
	f.seenOwners = append(f.seenOwners, owner)
	if c, ok := f.courses[owner+"/"+name]; ok {
		return c, nil
	}
	return nil, db.ErrCourseNotFound
}

func (f *fakeStore) SaveCourse(_ context.Context, course *db.StoredCourse) error {
	f.seenOwners = append(f.seenOwners, course.Owner)
	f.saved = course
	f.courses[course.Owner+"/"+course.Name] = course
	return nil
}

func (f *fakeStore) DeleteCourse(_ context.Context, owner, name string) error {
	f.seenOwners = append(f.seenOwners, owner)
	key := owner + "/" + name
	if _, ok := f.courses[key]; !ok {
		return db.ErrCourseNotFound
	}
	delete(f.courses, key)
	return nil
}

func ctxAs(email string) context.Context {
	return principal.WithUser(context.Background(), &model.User{Email: email})
}

// The whole isolation model rests on the owner coming from the authenticated
// principal. This proves it: every course operation scopes to the context user,
// and there is no argument that could override it.
func TestCourseOperationsScopeToThePrincipal(t *testing.T) {
	fs := newFakeStore()
	a := &App{db: fs}
	ctx := ctxAs("prof@hm.edu")

	if _, err := a.ImportCourseYAML(ctx, "mpd:\n  coursepath: mpd/s\n"); err != nil {
		t.Fatalf("import: %v", err)
	}
	if _, err := a.Courses(ctx); err != nil {
		t.Fatalf("courses: %v", err)
	}
	if _, err := a.Course(ctx, "mpd"); err != nil {
		t.Fatalf("course: %v", err)
	}
	_ = a.DeleteCourse(ctx, "mpd")

	for _, owner := range fs.seenOwners {
		if owner != "prof@hm.edu" {
			t.Errorf("a store call used owner %q, want prof@hm.edu — the owner must come from the principal", owner)
		}
	}
	if fs.saved == nil || fs.saved.Owner != "prof@hm.edu" {
		t.Errorf("saved course owner = %v, want prof@hm.edu", fs.saved)
	}
}

// Without an authenticated user every operation must refuse, never fall back to
// some default that could read another user's data.
func TestCourseOperationsRequireAuthentication(t *testing.T) {
	a := &App{db: newFakeStore()}
	ctx := context.Background() // no principal

	if _, err := a.Courses(ctx); err == nil {
		t.Error("Courses without a principal succeeded, want an error")
	}
	if _, err := a.Course(ctx, "mpd"); err == nil {
		t.Error("Course without a principal succeeded, want an error")
	}
	if _, err := a.ImportCourseYAML(ctx, "mpd:\n"); err == nil {
		t.Error("ImportCourseYAML without a principal succeeded, want an error")
	}
	if err := a.DeleteCourse(ctx, "mpd"); err == nil {
		t.Error("DeleteCourse without a principal succeeded, want an error")
	}
}

// Two users importing a course of the same name get two separate documents; one
// never sees or overwrites the other's.
func TestImportIsPerOwner(t *testing.T) {
	fs := newFakeStore()
	a := &App{db: fs}

	if _, err := a.ImportCourseYAML(ctxAs("a@hm.edu"), "mpd:\n  coursepath: a/s\n"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ImportCourseYAML(ctxAs("b@hm.edu"), "mpd:\n  coursepath: b/s\n"); err != nil {
		t.Fatal(err)
	}

	aCourses, _ := a.Courses(ctxAs("a@hm.edu"))
	bCourses, _ := a.Courses(ctxAs("b@hm.edu"))
	if len(aCourses) != 1 || len(bCourses) != 1 {
		t.Fatalf("each user should have one course, got a=%d b=%d", len(aCourses), len(bCourses))
	}
	if aCourses[0].Source.CoursePath == bCourses[0].Source.CoursePath {
		t.Error("the two users' courses share state — b overwrote a")
	}
}

func TestImportRejectsInlineSignKey(t *testing.T) {
	a := &App{db: newFakeStore()}
	yaml := "st:\n  coursepath: st/s\n  a1:\n    assignmentpath: a1\n    seeder:\n      cmd: /bin/true\n      signKey: \"-----BEGIN PGP PRIVATE KEY-----\"\n"

	_, err := a.ImportCourseYAML(ctxAs("prof@hm.edu"), yaml)
	if err == nil {
		t.Fatal("import with an inline signKey succeeded, want it rejected")
	}
}

func TestImportRejectsInvalidYAML(t *testing.T) {
	a := &App{db: newFakeStore()}
	if _, err := a.ImportCourseYAML(ctxAs("prof@hm.edu"), "a:\nb:\n"); err == nil {
		t.Error("import of a two-top-level-key document succeeded, want an error")
	}
}
