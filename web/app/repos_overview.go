package app

import (
	"context"
	"sort"
	"sync"

	"github.com/obcode/glabs/v3/config"
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

			ar := &AssignmentRepos{Name: name}
			cfg, err := a.resolveAssignmentConfig(ctx, course, name)
			if err != nil || cfg == nil {
				ar.Note = "abstrakt oder nicht auflösbar — übersprungen"
				out[i] = ar
				return
			}
			ar.Per = string(cfg.Per)
			targets := cfg.RepoTargets()
			ar.Targets = len(targets)

			existing, err := client.ExistingRepoNames(cfg)
			if err != nil {
				// Most likely the group does not exist yet (never generated); show
				// every target as missing rather than failing the whole overview.
				ar.Note = "GitLab-Gruppe nicht gefunden — vermutlich noch nicht generiert"
				ar.Repos, ar.Existing = matchRepos(targets, nil)
				out[i] = ar
				return
			}
			ar.Repos, ar.Existing = matchRepos(targets, existing)
			out[i] = ar
		}(i, name)
	}
	wg.Wait()
	return out, nil
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
