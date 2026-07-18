package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/obcode/glabs/v3/web/db"
)

func drainCheckEvents(ch <-chan CheckEvent) []CheckEvent {
	var evs []CheckEvent
	for ev := range ch {
		evs = append(evs, ev)
	}
	return evs
}

func TestCheckCourse_noTokenReturnsError(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}

	// The course resolves, but no token is stored → a clear error.
	_, err := a.CheckCourse(ctxAs(owner), "uc")
	if err == nil || !strings.Contains(err.Error(), "no GitLab token") {
		t.Fatalf("error = %v, want a missing-token error", err)
	}
}

func TestCheckCourse_courseNotFoundReturnsErrCourseNotFound(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}

	_, err := a.CheckCourse(ctxAs(owner), "nope")
	if !errors.Is(err, db.ErrCourseNotFound) {
		t.Fatalf("error = %v, want ErrCourseNotFound", err)
	}
}

func TestStreamCheckCourse_noTokenYieldsDoneWithError(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}

	ch, err := a.StreamCheckCourse(ctxAs(owner), "uc")
	if err != nil {
		t.Fatalf("StreamCheckCourse: %v", err)
	}
	evs := drainCheckEvents(ch)
	if len(evs) == 0 {
		t.Fatal("expected at least the final event")
	}
	last := evs[len(evs)-1]
	if !last.Done || !strings.Contains(last.Error, "no GitLab token") {
		t.Fatalf("final event = %+v, want done with a missing-token error", last)
	}
}
