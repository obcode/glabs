package app

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/principal"
)

// nameRe restricts course and assignment names to characters that are safe as a
// path segment (they become part of the GitLab group/project path).
var nameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// owner returns the email of the authenticated user, which scopes every course
// operation. It comes from the request context — set by the auth middleware —
// and never from a GraphQL argument, so a user can only ever act as themselves.
func owner(ctx context.Context) (string, error) {
	user := principal.UserFromContext(ctx)
	if user == nil || user.Email == "" {
		return "", fmt.Errorf("no authenticated user")
	}
	return user.Email, nil
}

// Courses returns the caller's own courses.
func (a *App) Courses(ctx context.Context) ([]*db.StoredCourse, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	return a.db.CoursesOf(ctx, o)
}

// Course returns one of the caller's own courses.
func (a *App) Course(ctx context.Context, name string) (*db.StoredCourse, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	return a.db.CourseOf(ctx, o, name)
}

// ImportCourseYAML parses an uploaded course file and stores it for the caller,
// keeping the original bytes so a later download returns exactly what was
// uploaded. Importing a course whose name already exists replaces it.
//
// An inline seeder signKey is rejected: it is a private key, and storing it would
// mean sealing it, hiding it in GraphQL, and excluding it from dumps — four
// leaks for a feature no course file uses. signKeyFile (a path) is fine.
func (a *App) ImportCourseYAML(ctx context.Context, yaml string) (*db.StoredCourse, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}

	source, _, err := config.DecodeCourse([]byte(yaml))
	if err != nil {
		return nil, err
	}
	if err := rejectInlineSignKeys(source); err != nil {
		return nil, err
	}

	now := time.Now()
	existing, err := a.db.CourseOf(ctx, o, source.Name)
	importedAt := now
	if err == nil {
		importedAt = existing.ImportedAt
	}

	stored := &db.StoredCourse{
		Owner:      o,
		Name:       source.Name,
		Source:     source,
		RawYAML:    []byte(yaml),
		ImportedAt: importedAt,
		UpdatedAt:  now,
	}
	if err := a.db.SaveCourse(ctx, stored); err != nil {
		return nil, err
	}
	return stored, nil
}

// CreateCourse creates a new, empty course from scratch for the caller. It fails
// if the caller already has a course by that name — assignments are added
// afterwards with SetAssignment.
func (a *App) CreateCourse(ctx context.Context, name, coursePath, semesterPath string, useCoursenameAsPrefix, useEmailDomainAsSuffix bool) (*db.StoredCourse, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if !nameRe.MatchString(name) {
		return nil, fmt.Errorf("invalid course name %q: use only letters, digits, '.', '-' and '_'", name)
	}

	existing, err := a.db.CourseOf(ctx, o, name)
	if err != nil && !errors.Is(err, db.ErrCourseNotFound) {
		return nil, err
	}
	if existing != nil {
		return nil, fmt.Errorf("a course named %q already exists", name)
	}

	ueds := useEmailDomainAsSuffix
	source := &config.CourseSource{
		Name:                   name,
		CoursePath:             strings.TrimSpace(coursePath),
		SemesterPath:           strings.TrimSpace(semesterPath),
		UseCoursenameAsPrefix:  useCoursenameAsPrefix,
		UseEmailDomainAsSuffix: &ueds,
	}
	raw, err := config.EncodeCourse(source)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	stored := &db.StoredCourse{
		Owner:      o,
		Name:       name,
		Source:     source,
		RawYAML:    raw,
		ImportedAt: now,
		UpdatedAt:  now,
	}
	if err := a.db.SaveCourse(ctx, stored); err != nil {
		return nil, err
	}
	return stored, nil
}

// DeleteCourse removes one of the caller's own courses.
func (a *App) DeleteCourse(ctx context.Context, name string) error {
	o, err := owner(ctx)
	if err != nil {
		return err
	}
	return a.db.DeleteCourse(ctx, o, name)
}

// CourseYAML returns the course as a downloadable YAML file: the original bytes
// if they are still current, otherwise a re-encoding of the stored source.
func (a *App) CourseYAML(ctx context.Context, name string) ([]byte, error) {
	course, err := a.Course(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(course.RawYAML) > 0 {
		return course.RawYAML, nil
	}
	return config.EncodeCourse(course.Source)
}

// CourseLint returns the lint findings for one of the caller's own courses.
func (a *App) CourseLint(ctx context.Context, name string) ([]config.Finding, error) {
	course, err := a.Course(ctx, name)
	if err != nil {
		return nil, err
	}
	// Re-derive the decode result from the source so lint sees the same unknown
	// and legacy keys. Decoding the stored raw bytes keeps it faithful to what
	// the user uploaded.
	var decoded *config.DecodeResult
	if len(course.RawYAML) > 0 {
		if _, d, err := config.DecodeCourse(course.RawYAML); err == nil {
			decoded = d
		}
	}
	if decoded == nil {
		decoded = &config.DecodeResult{}
	}
	return config.Lint(course.Source, decoded), nil
}

func rejectInlineSignKeys(course *config.CourseSource) error {
	for name, a := range course.Assignments {
		if a.Seeder != nil && a.Seeder.SignKey != "" {
			return fmt.Errorf("assignment %q has an inline seeder signKey: a private key must not be uploaded; use signKeyFile instead", name)
		}
	}
	return nil
}
