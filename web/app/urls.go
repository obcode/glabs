package app

import (
	"context"

	"github.com/obcode/glabs/v3/config"
)

// AssignmentURLs are the repository URLs for one assignment: the assignment-level
// group URL plus one entry per student or per group. Derived purely from the
// resolved configuration — no GitLab token or API call is involved.
type AssignmentURLs struct {
	// Per is "student" or "group".
	Per string
	// GroupURL is the assignment-level group URL where all the repos live.
	GroupURL string
	// Repos is one URL per student/group repository.
	Repos []config.RepoURL
}

// AssignmentURLs returns the repository URLs for one assignment of one of the
// caller's own courses. It returns nil (no error) when the course has no such
// assignment, or when the assignment cannot be resolved (e.g. an abstract base) —
// in both cases there simply are no URLs. ErrCourseNotFound propagates when the
// course itself is not the caller's.
func (a *App) AssignmentURLs(ctx context.Context, course, name string) (*AssignmentURLs, error) {
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, err
	}

	src, ok := stored.Source.Assignments[name]
	if !ok || src == nil {
		return nil, nil
	}

	// Resolve from the stored bytes so the URLs match the CLI exactly (the same
	// inheritance, roster resolution and path normalization). Fall back to
	// re-encoding the source if the raw bytes are absent.
	bytes := stored.RawYAML
	if len(bytes) == 0 {
		if encoded, err := config.EncodeCourse(stored.Source); err == nil {
			bytes = encoded
		}
	}
	cfg, err := config.ResolveAssignmentFromBytes(bytes, course, name, config.Globals{GitlabHost: a.gitlabHost})
	if err != nil {
		// Not resolvable (abstract base, missing parent, cycle) → no URLs.
		return nil, nil
	}

	return &AssignmentURLs{
		Per:      string(cfg.Per),
		GroupURL: cfg.URL,
		Repos:    cfg.RepoURLs(),
	}, nil
}
