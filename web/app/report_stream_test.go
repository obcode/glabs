package app

import (
	"strings"
	"testing"
)

func drainEvents(ch <-chan ReportEvent) []ReportEvent {
	var evs []ReportEvent
	for ev := range ch {
		evs = append(evs, ev)
	}
	return evs
}

func TestStreamAssignmentReport_missingAssignmentYieldsDoneNoReport(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}

	ch, err := a.StreamAssignmentReport(ctxAs(owner), "uc", "nope")
	if err != nil {
		t.Fatalf("StreamAssignmentReport: %v", err)
	}
	evs := drainEvents(ch)
	if len(evs) != 1 || !evs[0].Done || evs[0].Report != nil || evs[0].Error != "" {
		t.Fatalf("events = %+v, want a single done event with no report/error", evs)
	}
}

func TestStreamAssignmentReport_abstractBaseYieldsDoneNoReport(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/tc"] = storedCourse(t, owner, tcCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}

	ch, err := a.StreamAssignmentReport(ctxAs(owner), "tc", "base")
	if err != nil {
		t.Fatalf("StreamAssignmentReport: %v", err)
	}
	evs := drainEvents(ch)
	if len(evs) != 1 || !evs[0].Done || evs[0].Report != nil {
		t.Fatalf("events = %+v, want a single done event with no report", evs)
	}
}

func TestStreamAssignmentReport_noTokenYieldsDoneWithError(t *testing.T) {
	const owner = "prof@hm.edu"
	fs := newFakeStore()
	fs.courses[owner+"/uc"] = storedCourse(t, owner, urlsCourse)
	a := &App{db: fs, gitlabHost: "https://gl", sealer: testSealer(t)}

	ch, err := a.StreamAssignmentReport(ctxAs(owner), "uc", "blatt1")
	if err != nil {
		t.Fatalf("StreamAssignmentReport: %v", err)
	}
	evs := drainEvents(ch)
	if len(evs) == 0 {
		t.Fatal("expected at least the final event")
	}
	last := evs[len(evs)-1]
	if !last.Done || !strings.Contains(last.Error, "no GitLab token") {
		t.Fatalf("final event = %+v, want done with a missing-token error", last)
	}
}
