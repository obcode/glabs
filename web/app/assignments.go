package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

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

// ValidationResult reports whether a draft assignment is saveable and — when it
// resolves — its preview.
type ValidationResult struct {
	OK           bool
	Errors       []string
	Resolved     string
	ResolveError string
}

// applyDraft returns a copy of src with the draft's core-field values applied.
// An empty value unsets a field (so it inherits). Only the schema's core fields
// are editable here; nested blocks (mergeRequest, branches, …) are carried over
// untouched.
func applyDraft(src *config.AssignmentSource, draft map[string]string) *config.AssignmentSource {
	c := *src
	for key, val := range draft {
		switch key {
		case "extends":
			c.Extends = val
		case "abstract":
			c.Abstract = val == "true"
		case "per":
			c.Per = val
		case "accesslevel":
			c.AccessLevel = val
		case "description":
			c.Description = val
		case "assignmentpath":
			c.AssignmentPath = val
		case "containerRegistry":
			c.ContainerRegistry = val == "true"
		}
	}
	return &c
}

// courseWithDraft returns a shallow copy of the stored course source with the
// named assignment replaced by the drafted version, without mutating the stored
// source (its Assignments map is copied before the swap).
func courseWithDraft(src *config.CourseSource, name string, drafted *config.AssignmentSource) *config.CourseSource {
	newCourse := *src
	assignments := make(map[string]*config.AssignmentSource, len(src.Assignments))
	for k, v := range src.Assignments {
		assignments[k] = v
	}
	assignments[name] = drafted
	newCourse.Assignments = assignments
	return &newCourse
}

// validateDrafted runs the drafted course through the real resolver. An abstract
// draft is valid but has no preview; a concrete draft that does not resolve is a
// hard error.
func (a *App) validateDrafted(newCourse *config.CourseSource, course, name string, drafted *config.AssignmentSource) *ValidationResult {
	if drafted.Abstract {
		return &ValidationResult{OK: true, ResolveError: "abstrakte Basis — keine aufgelöste Vorschau"}
	}
	bytes, err := config.EncodeCourse(newCourse)
	if err != nil {
		return &ValidationResult{OK: false, Errors: []string{err.Error()}}
	}
	cfg, err := config.ResolveAssignmentFromBytes(bytes, course, name, config.Globals{GitlabHost: a.gitlabHost})
	if err != nil {
		return &ValidationResult{OK: false, Errors: []string{err.Error()}, ResolveError: err.Error()}
	}
	return &ValidationResult{OK: true, Resolved: cfg.Show()}
}

// draftedAssignment loads the caller's course and applies the draft to a copy of
// the named assignment, returning the new (unsaved) course source and the drafted
// assignment.
func (a *App) draftedAssignment(ctx context.Context, course, name string, draft map[string]string) (*config.CourseSource, *config.AssignmentSource, error) {
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, nil, err
	}
	orig, ok := stored.Source.Assignments[name]
	if !ok || orig == nil {
		return nil, nil, fmt.Errorf("assignment %q not found in course %q", name, course)
	}
	drafted := applyDraft(orig, draft)
	return courseWithDraft(stored.Source, name, drafted), drafted, nil
}

// ValidateAssignmentDraft validates a draft against the real resolver without
// saving.
func (a *App) ValidateAssignmentDraft(ctx context.Context, course, name string, draft map[string]string) (*ValidationResult, error) {
	newCourse, drafted, err := a.draftedAssignment(ctx, course, name, draft)
	if err != nil {
		return nil, err
	}
	return a.validateDrafted(newCourse, course, name, drafted), nil
}

// SetAssignment applies a draft to one of the caller's assignments: it validates,
// rejects a concrete draft that does not resolve, then persists — re-marshalling
// the whole course YAML (a real edit gives up the verbatim original bytes).
func (a *App) SetAssignment(ctx context.Context, course, name string, draft map[string]string) (*AssignmentView, error) {
	stored, err := a.Course(ctx, course)
	if err != nil {
		return nil, err
	}
	orig, ok := stored.Source.Assignments[name]
	if !ok || orig == nil {
		return nil, fmt.Errorf("assignment %q not found in course %q", name, course)
	}
	drafted := applyDraft(orig, draft)
	newCourse := courseWithDraft(stored.Source, name, drafted)

	if vr := a.validateDrafted(newCourse, course, name, drafted); !vr.OK {
		return nil, fmt.Errorf("assignment %q is not valid: %s", name, strings.Join(vr.Errors, "; "))
	}

	raw, err := config.EncodeCourse(newCourse)
	if err != nil {
		return nil, err
	}
	stored.Source = newCourse
	stored.RawYAML = raw
	stored.UpdatedAt = time.Now()
	if err := a.db.SaveCourse(ctx, stored); err != nil {
		return nil, err
	}
	return a.Assignment(ctx, course, name)
}
