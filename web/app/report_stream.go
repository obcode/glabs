package app

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/obcode/glabs/v3/gitlab/report"
	"github.com/obcode/glabs/v3/reporter"
)

// ReportEvent is one item in the assignment-report stream: either a progress
// message while the report is being fetched, or the single final event carrying
// the finished report (or an error).
type ReportEvent struct {
	Message string
	Done    bool
	Report  *report.Reports
	Error   string
}

// StreamAssignmentReport generates the report for one assignment and streams the
// GitLab client's progress as it goes, ending with exactly one done event that
// carries the finished report (or an error). The returned channel is closed when
// generation finishes or ctx is cancelled.
//
// Like AssignmentReport, a missing or unresolvable assignment yields a done event
// with a nil report (no error); a missing token or an unreachable GitLab yields a
// done event whose Error is set — surfaced to the client rather than tearing down
// the subscription.
func (a *App) StreamAssignmentReport(ctx context.Context, course, name string) (<-chan ReportEvent, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}

	events := make(chan ReportEvent)

	cfg, err := a.resolveAssignmentConfig(ctx, course, name)
	if err != nil {
		return nil, err
	}
	if cfg == nil {
		// Nothing to fetch — one done event with no report.
		go func() {
			defer close(events)
			sendEvent(ctx, events, ReportEvent{Done: true})
		}()
		return events, nil
	}

	rep := &streamReporter{emit: func(msg string) { sendEvent(ctx, events, ReportEvent{Message: msg}) }}
	client, err := a.gitlabClientFor(ctx, o, rep)
	if err != nil {
		go func() {
			defer close(events)
			sendEvent(ctx, events, ReportEvent{Done: true, Error: err.Error()})
		}()
		return events, nil
	}

	go func() {
		defer close(events)
		// ReportData drives the streamReporter synchronously as it works, so its
		// progress lines flow through `events` before this final event.
		rep, err := client.ReportData(cfg)
		ev := ReportEvent{Done: true}
		if err != nil {
			ev.Error = err.Error()
		} else {
			ev.Report = rep
		}
		sendEvent(ctx, events, ev)
	}()

	return events, nil
}

func sendEvent(ctx context.Context, ch chan<- ReportEvent, ev ReportEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*m")

// streamReporter is a reporter.Reporter that forwards the GitLab client's progress
// to an emit callback, stripping ANSI color codes so the browser gets clean lines.
// The callback owns delivery (a ctx-aware channel send), so a disconnected client
// never blocks the fetch. It is shared by the report and check streams.
type streamReporter struct {
	emit func(msg string)
}

func (s *streamReporter) send(msg string) {
	msg = strings.TrimSpace(ansiRE.ReplaceAllString(msg, ""))
	if msg == "" {
		return
	}
	s.emit(msg)
}

func (s *streamReporter) Printf(format string, a ...any) { s.send(fmt.Sprintf(format, a...)) }
func (s *streamReporter) Println(a ...any)               { s.send(fmt.Sprintln(a...)) }
func (s *streamReporter) Task(description string) reporter.Task {
	s.send(description)
	return &streamTask{s}
}

type streamTask struct{ s *streamReporter }

func (t *streamTask) Update(message string) { t.s.send(message) }
func (t *streamTask) Done(message string)   { t.s.send(message) }
func (t *streamTask) Fail(message string)   { t.s.send(message) }
