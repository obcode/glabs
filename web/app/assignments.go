package app

import (
	"context"
	"strconv"

	"github.com/obcode/glabs/v3/config"
)

// FieldValue is one field's own (source) value, stringified and keyed by the
// same key as FieldMeta so the GUI can pre-fill the schema-driven form.
type FieldValue struct {
	Key   string
	Value string
}

// assignmentOwn extracts the source values of the schema's core fields from an
// AssignmentSource, keyed by FieldMeta.key. Booleans become "true"/"false".
func assignmentOwn(src *config.AssignmentSource) []FieldValue {
	return []FieldValue{
		{Key: "extends", Value: src.Extends},
		{Key: "abstract", Value: strconv.FormatBool(src.Abstract)},
		{Key: "per", Value: src.Per},
		{Key: "accesslevel", Value: src.AccessLevel},
		{Key: "description", Value: src.Description},
		{Key: "assignmentpath", Value: src.AssignmentPath},
		{Key: "containerRegistry", Value: strconv.FormatBool(src.ContainerRegistry)},
	}
}

// AssignmentView is one assignment in source (own) form plus its resolved
// preview — the same rendering `glabs show` produces. It makes the inheritance
// visible in the browser exactly as the CLI's confirmation gate does.
type AssignmentView struct {
	Course   string
	Name     string
	Extends  string
	Abstract bool
	// Own holds the assignment's own (source) field values, keyed by
	// FieldMeta.key, for pre-filling the editor form.
	Own []FieldValue
	// Resolved is the Show() rendering (may contain ANSI). Empty when the
	// assignment cannot be resolved (then ResolveError explains why).
	Resolved string
	// ResolveError is set when resolution fails — e.g. an abstract base, a
	// missing parent, or an `extends` cycle. Not a fault: an abstract base is a
	// valid document that simply has no resolved form.
	ResolveError string
}

// Assignment returns the source values and resolved preview for one assignment
// of one of the caller's own courses. It returns nil (no error) when the course
// exists but has no assignment by that name; ErrCourseNotFound propagates when
// the course itself is not the caller's.
func (a *App) Assignment(ctx context.Context, course, name string) (*AssignmentView, error) {
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, err
	}

	src, ok := stored.Source.Assignments[name]
	if !ok || src == nil {
		return nil, nil
	}

	view := &AssignmentView{
		Course:   course,
		Name:     name,
		Extends:  src.Extends,
		Abstract: src.Abstract,
		Own:      assignmentOwn(src),
	}

	// Resolve from the stored bytes so the preview matches the CLI exactly
	// (inheritance, legacy keys, lowercasing). Fall back to re-encoding the
	// source if the raw bytes are absent.
	bytes := stored.RawYAML
	if len(bytes) == 0 {
		if encoded, err := config.EncodeCourse(stored.Source); err == nil {
			bytes = encoded
		}
	}
	cfg, err := config.ResolveAssignmentFromBytes(bytes, course, name, config.Globals{GitlabHost: a.gitlabHost})
	if err != nil {
		view.ResolveError = err.Error()
		return view, nil
	}
	view.Resolved = cfg.Show()
	return view, nil
}
