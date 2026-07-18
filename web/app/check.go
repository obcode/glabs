package app

import (
	"context"
	"fmt"

	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/gitlab"
	"github.com/obcode/glabs/v3/reporter"
	"gopkg.in/yaml.v3"
)

// resolveCourseConfig resolves one of the caller's courses into a CourseConfig
// (its roster resolved into Students/Groups) from the stored bytes, so the result
// matches the CLI exactly. ErrCourseNotFound propagates when the course is not the
// caller's.
func (a *App) resolveCourseConfig(ctx context.Context, courseName string) (*config.CourseConfig, error) {
	stored, err := a.Course(ctx, courseName)
	if err != nil {
		return nil, err
	}
	bytes := stored.RawYAML
	if len(bytes) == 0 {
		if encoded, err := config.EncodeCourse(stored.Source); err == nil {
			bytes = encoded
		}
	}
	var top map[string]any
	if err := yaml.Unmarshal(bytes, &top); err != nil {
		return nil, fmt.Errorf("cannot parse stored course: %w", err)
	}
	body, ok := top[courseName]
	if !ok {
		return nil, fmt.Errorf("stored course %q has an unexpected shape", courseName)
	}
	return config.ResolveCourse(courseName, body)
}

// CheckCourse resolves the course roster against GitLab (one-shot, no progress),
// using the caller's stored token.
func (a *App) CheckCourse(ctx context.Context, courseName string) (*gitlab.CheckResult, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	cfg, err := a.resolveCourseConfig(ctx, courseName)
	if err != nil {
		return nil, err
	}
	client, err := a.gitlabClientFor(ctx, o, reporter.NewDiscardReporter())
	if err != nil {
		return nil, err
	}
	return client.CheckCourseData(cfg), nil
}

// CheckEvent is one item in the course-check stream: a progress message while the
// roster is checked, or the single final done event carrying the result (or an
// error, e.g. no token stored).
type CheckEvent struct {
	Message string
	Done    bool
	Result  *gitlab.CheckResult
	Error   string
}

// StreamCheckCourse checks the course roster against GitLab and streams progress
// per student, ending with one done event carrying the result. A missing token or
// an unreachable GitLab yields a done event whose Error is set.
func (a *App) StreamCheckCourse(ctx context.Context, courseName string) (<-chan CheckEvent, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	cfg, err := a.resolveCourseConfig(ctx, courseName)
	if err != nil {
		return nil, err
	}

	events := make(chan CheckEvent)
	rep := &streamReporter{emit: func(msg string) { sendCheckEvent(ctx, events, CheckEvent{Message: msg}) }}
	client, err := a.gitlabClientFor(ctx, o, rep)
	if err != nil {
		go func() {
			defer close(events)
			sendCheckEvent(ctx, events, CheckEvent{Done: true, Error: err.Error()})
		}()
		return events, nil
	}

	go func() {
		defer close(events)
		res := client.CheckCourseData(cfg) // drives the streamReporter synchronously
		sendCheckEvent(ctx, events, CheckEvent{Done: true, Result: res})
	}()
	return events, nil
}

func sendCheckEvent(ctx context.Context, ch chan<- CheckEvent, ev CheckEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}
