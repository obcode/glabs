package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/obcode/glabs/v3/config"
	"gopkg.in/yaml.v3"
)

// ImportAssignmentYAML upserts a single assignment into one of the caller's
// courses from a YAML snippet in the same keyed form it has in a course file:
//
//	blatt3:
//	  per: student
//	  accesslevel: developer
//	  startercode:
//	    url: git@gitlab.lrz.de:...
//
// The snippet's single top-level key is the assignment name. The block is merged
// into the stored course, decoded and validated through the same resolver the
// editor uses (a concrete assignment that does not resolve is rejected), then
// persisted — re-marshalling the course YAML, exactly like SetAssignment.
func (a *App) ImportAssignmentYAML(ctx context.Context, course, assignmentYAML string) (*AssignmentView, error) {
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, err
	}

	// The snippet must have exactly one top-level key — the assignment name.
	var snippet map[string]any
	if err := yaml.Unmarshal([]byte(assignmentYAML), &snippet); err != nil {
		return nil, fmt.Errorf("cannot parse assignment YAML: %w", err)
	}
	switch len(snippet) {
	case 1:
	case 0:
		return nil, fmt.Errorf("assignment YAML is empty: expected a single top-level key naming the assignment")
	default:
		return nil, fmt.Errorf("assignment YAML has %d top-level keys: expected exactly one, naming the assignment", len(snippet))
	}
	var name string
	var body any
	for k, v := range snippet {
		name, body = k, v
	}
	name = strings.TrimSpace(name)
	if !nameRe.MatchString(name) {
		return nil, fmt.Errorf("invalid assignment name %q: use only letters, digits, '.', '-' and '_'", name)
	}

	// Merge the block into the stored course's body, then decode the whole course
	// through the same path a course import uses.
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
	courseBody, ok := top[course].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("stored course %q has an unexpected shape", course)
	}
	courseBody[name] = body

	source, _, err := config.DecodeCourseBody(course, courseBody)
	if err != nil {
		return nil, fmt.Errorf("invalid assignment: %w", err)
	}
	if err := rejectInlineSignKeys(source); err != nil {
		return nil, err
	}

	// Reject a concrete assignment that does not resolve (an abstract base is fine).
	if vr := a.validateDrafted(source, course, name, source.Assignments[name]); !vr.OK {
		return nil, fmt.Errorf("assignment %q is not valid: %s", name, strings.Join(vr.Errors, "; "))
	}

	raw, err := config.EncodeCourse(source)
	if err != nil {
		return nil, err
	}
	stored.Source = source
	stored.RawYAML = raw
	stored.UpdatedAt = time.Now()
	if err := a.db.SaveCourse(ctx, stored); err != nil {
		return nil, err
	}
	return a.Assignment(ctx, course, name)
}
