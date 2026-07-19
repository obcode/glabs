package app

import (
	"context"
	"sort"
	"sync"

	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/gitlab"
	"github.com/obcode/glabs/v3/reporter"
)

// repoOverviewConcurrency bounds how many assignments are queried at once.
const repoOverviewConcurrency = 6

// RepoStatus is one target repository and whether it actually exists in GitLab.
type RepoStatus struct {
	For    string
	Repo   string
	URL    string
	Exists bool
}

// AssignmentRepos summarises, for one assignment, how many of its target repos have
// been generated. Note carries a reason when the assignment could not be checked
// (abstract/unresolvable, or its GitLab group does not exist yet).
type AssignmentRepos struct {
	Name     string
	Per      string
	Targets  int
	Existing int
	Repos    []RepoStatus
	Note     string
}

// CourseRepoOverview reports, per assignment of one of the caller's courses, which
// target repositories actually exist in GitLab — so the owner can see at a glance
// what has been generated and who is missing. It uses one group listing per
// assignment (not a lookup per student) and queries assignments concurrently. It
// needs the caller's stored GitLab token.
func (a *App) CourseRepoOverview(ctx context.Context, course string) ([]*AssignmentRepos, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, err
	}
	client, err := a.gitlabClientFor(ctx, o, reporter.NewDiscardReporter())
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(stored.Source.Assignments))
	for name := range stored.Source.Assignments {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]*AssignmentRepos, len(names))
	sem := make(chan struct{}, repoOverviewConcurrency)
	var wg sync.WaitGroup
	for i, name := range names {
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = a.assignmentRepos(ctx, client, course, name)
		}(i, name)
	}
	wg.Wait()
	return out, nil
}

// assignmentRepos computes the repo status of one assignment: it resolves the
// config, lists the group's existing projects and matches them against the
// targets. An abstract/unresolvable assignment, or a group that does not exist yet,
// yields a Note rather than an error, so one gap never fails the whole overview.
func (a *App) assignmentRepos(ctx context.Context, client *gitlab.Client, course, name string) *AssignmentRepos {
	ar := &AssignmentRepos{Name: name}
	cfg, err := a.resolveAssignmentConfig(ctx, course, name)
	if err != nil || cfg == nil {
		ar.Note = "abstrakt oder nicht auflösbar — übersprungen"
		return ar
	}
	ar.Per = string(cfg.Per)
	targets := cfg.RepoTargets()
	ar.Targets = len(targets)

	existing, err := client.ExistingRepoNames(cfg)
	if err != nil {
		// Most likely the group does not exist yet (never generated); show every
		// target as missing rather than failing the whole overview.
		ar.Note = "GitLab-Gruppe nicht gefunden — vermutlich noch nicht generiert"
		ar.Repos, ar.Existing = matchRepos(targets, nil)
		return ar
	}
	ar.Repos, ar.Existing = matchRepos(targets, existing)
	return ar
}

// RepoOverviewEvent is one streamed item of a course repo overview: a completed
// assignment as it finishes, then a final Done event. Error is set (with Done) when
// the whole overview cannot start — e.g. no stored GitLab token.
type RepoOverviewEvent struct {
	Assignment *AssignmentRepos
	Total      int
	Done       bool
	Error      string
}

// StreamCourseRepoOverview runs the same overview as CourseRepoOverview but emits
// each assignment as soon as it is checked, so the GUI shows progress instead of a
// frozen page. The channel is always returned; a missing token surfaces as a final
// Done event carrying Error rather than a synchronous failure.
func (a *App) StreamCourseRepoOverview(ctx context.Context, course string) (<-chan RepoOverviewEvent, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(stored.Source.Assignments))
	for name := range stored.Source.Assignments {
		names = append(names, name)
	}
	sort.Strings(names)
	total := len(names)

	events := make(chan RepoOverviewEvent)
	client, err := a.gitlabClientFor(ctx, o, reporter.NewDiscardReporter())
	if err != nil {
		go func() {
			defer close(events)
			sendRepoEvent(ctx, events, RepoOverviewEvent{Done: true, Total: total, Error: err.Error()})
		}()
		return events, nil
	}

	go func() {
		defer close(events)
		sem := make(chan struct{}, repoOverviewConcurrency)
		var wg sync.WaitGroup
		for _, name := range names {
			wg.Add(1)
			go func(name string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				ar := a.assignmentRepos(ctx, client, course, name)
				sendRepoEvent(ctx, events, RepoOverviewEvent{Assignment: ar, Total: total})
			}(name)
		}
		wg.Wait()
		sendRepoEvent(ctx, events, RepoOverviewEvent{Done: true, Total: total})
	}()
	return events, nil
}

func sendRepoEvent(ctx context.Context, ch chan<- RepoOverviewEvent, ev RepoOverviewEvent) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}

// matchRepos pairs each target with whether its repo exists, and counts the
// existing ones. A nil existing set means none exist.
func matchRepos(targets []config.RepoTarget, existing map[string]bool) ([]RepoStatus, int) {
	repos := make([]RepoStatus, 0, len(targets))
	count := 0
	for _, t := range targets {
		ex := existing[t.Repo]
		if ex {
			count++
		}
		repos = append(repos, RepoStatus{For: t.For, Repo: t.Repo, URL: t.URL, Exists: ex})
	}
	return repos, count
}
