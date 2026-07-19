package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/obcode/glabs/v3/web/zpa"
)

const rosterCourse = `sc:
  coursepath: sc/sem
  students:
    - a.mueller@hm.edu
    - z.unknown@hm.edu
  blatt01:
    per: student
`

func TestCourseStudents_enrichesFromZPA(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/sc"] = storedCourse(t, owner, rosterCourse)

	// Mock ZPA: knows a.mueller, does not know z.unknown.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var out []*zpa.Student
		if r.URL.Query().Get("ask") == "a.mueller@hm.edu" {
			out = []*zpa.Student{{Mtknr: "42", FirstName: "Anna", LastName: "Müller", Email: "a.mueller@hm.edu", Gender: "f", Group: "IF1"}}
		}
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(srv.Close)

	a := &App{db: fs, zpa: zpa.New(zpa.Config{BaseURL: srv.URL, Token: "x"})}
	students, err := a.CourseStudents(ctxAs(owner), "sc")
	if err != nil {
		t.Fatalf("CourseStudents: %v", err)
	}
	if len(students) != 2 {
		t.Fatalf("got %d students, want 2", len(students))
	}
	// Enriched student sorts first.
	if s := students[0]; !s.Found || s.LastName != "Müller" || s.Group != "IF1" || s.Mtknr != "42" {
		t.Errorf("students[0] = %+v, want the enriched Müller", s)
	}
	// Unknown sinks to the bottom, un-enriched.
	if s := students[1]; s.Found || s.Email != "z.unknown@hm.edu" {
		t.Errorf("students[1] = %+v, want the un-enriched z.unknown", s)
	}
}

// Without a ZPA client the roster still comes back, just un-enriched.
func TestCourseStudents_noZPAReturnsRosterOnly(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/sc"] = storedCourse(t, owner, rosterCourse)

	a := &App{db: fs} // no zpa
	students, err := a.CourseStudents(ctxAs(owner), "sc")
	if err != nil {
		t.Fatalf("CourseStudents: %v", err)
	}
	if len(students) != 2 {
		t.Fatalf("got %d students, want 2", len(students))
	}
	for _, s := range students {
		if s.Found {
			t.Errorf("student %s should be un-enriched without ZPA", s.Email)
		}
	}
}

// The roster read is owner-scoped: another user's course is invisible.
func TestCourseStudents_requiresAuthentication(t *testing.T) {
	a := &App{db: newFakeStore()}
	if _, err := a.CourseStudents(context.Background(), "sc"); err == nil {
		t.Error("CourseStudents without a principal should fail")
	}
}
