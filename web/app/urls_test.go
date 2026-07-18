package app

import (
	"strings"
	"testing"
)

const urlsCourse = `uc:
  coursepath: uc/sem
  semesterpath: ss26
  usecoursenameasprefix: true
  blatt1:
    per: student
    accesslevel: developer
    students:
      - a@hm.edu
      - b@hm.edu
`

func TestAssignmentURLs_PerStudent(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gitlab.example"}
	ctx := ctxAs(owner)

	u, err := a.AssignmentURLs(ctx, "uc", "blatt1")
	if err != nil {
		t.Fatalf("AssignmentURLs: %v", err)
	}
	if u == nil {
		t.Fatal("expected URLs for blatt1")
	}
	if u.Per != "student" {
		t.Errorf("Per = %q, want student", u.Per)
	}
	if !strings.HasPrefix(u.GroupURL, "https://gitlab.example/") {
		t.Errorf("GroupURL = %q, want it under the gitlab host", u.GroupURL)
	}
	if len(u.Repos) != 2 {
		t.Fatalf("Repos len = %d, want 2", len(u.Repos))
	}
	for _, r := range u.Repos {
		if r.For == "" {
			t.Errorf("repo For is empty: %+v", r)
		}
		if !strings.HasPrefix(r.URL, u.GroupURL+"/") {
			t.Errorf("repo URL %q is not under the group URL %q", r.URL, u.GroupURL)
		}
	}
}

func TestAssignmentURLs_missingAssignmentReturnsNil(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gitlab.example"}

	u, err := a.AssignmentURLs(ctxAs(owner), "uc", "nope")
	if err != nil {
		t.Fatalf("AssignmentURLs: %v", err)
	}
	if u != nil {
		t.Errorf("expected nil for a missing assignment, got %+v", u)
	}
}

func TestAssignmentURLs_abstractBaseReturnsNil(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs, gitlabHost: "https://gl"}

	// An abstract base does not resolve → no URLs (nil, no error).
	u, err := a.AssignmentURLs(ctxAs(owner), "tc", "base")
	if err != nil {
		t.Fatalf("AssignmentURLs: %v", err)
	}
	if u != nil {
		t.Errorf("expected nil for an abstract base, got %+v", u)
	}
}
