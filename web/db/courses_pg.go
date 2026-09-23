package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/obcode/glabs/v3/config"
	"github.com/obcode/glabs/v3/web/db/sqlc"
)

// courseFormatVersion is the format the json tags on config.CourseSource
// currently describe, written into every row and checked on every read.
//
// Checking rather than trusting is the lesson the same pattern taught in
// plexams: the tags ARE the storage format, so renaming one silently changes
// what a row means, and the value would come back as a zero with nothing
// reporting an error. A row written by a NEWER binary is refused here instead --
// which is what happens if a deploy is rolled back past a format change.
const courseFormatVersion = 1

// storedCourseFromRow maps a row to the DTO the application works with.
func storedCourseFromRow(row sqlc.Course) (*StoredCourse, error) {
	if row.FormatVersion > courseFormatVersion {
		return nil, fmt.Errorf(
			"course %s/%s was written in source format %d, but this build only understands %d "+
				"-- it was stored by a newer glabs-web",
			row.Owner, row.Name, row.FormatVersion, courseFormatVersion)
	}

	var source config.CourseSource
	if err := json.Unmarshal(row.Source, &source); err != nil {
		return nil, fmt.Errorf("cannot decode course %s/%s: %w", row.Owner, row.Name, err)
	}

	return &StoredCourse{
		Owner:      row.Owner,
		Name:       row.Name,
		Source:     &source,
		RawYAML:    emptyToNil(row.RawYaml),
		ImportedAt: row.ImportedAt,
		UpdatedAt:  row.UpdatedAt,
	}, nil
}

func storedCoursesFromRows(rows []sqlc.Course) ([]*StoredCourse, error) {
	out := make([]*StoredCourse, 0, len(rows))
	for _, row := range rows {
		course, err := storedCourseFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, course)
	}
	return out, nil
}

// CoursesOf returns the courses owned by the given user, sorted by name.
func (db *PG) CoursesOf(ctx context.Context, owner string) ([]*StoredCourse, error) {
	rows, err := db.queries.CoursesOf(ctx, owner)
	if err != nil {
		return nil, fmt.Errorf("cannot list courses: %w", err)
	}
	return storedCoursesFromRows(rows)
}

// CourseOf returns one course owned by the given user, or ErrCourseNotFound.
//
// It deliberately does not distinguish "does not exist" from "belongs to someone
// else": to one user, another user's course simply is not there.
func (db *PG) CourseOf(ctx context.Context, owner, name string) (*StoredCourse, error) {
	row, err := db.queries.CourseOf(ctx, sqlc.CourseOfParams{Owner: owner, Name: name})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCourseNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read course %s: %w", name, err)
	}
	return storedCourseFromRow(row)
}

// SaveCourse inserts or replaces a course for its owner.
func (db *PG) SaveCourse(ctx context.Context, course *StoredCourse) error {
	if course.Owner == "" {
		return fmt.Errorf("refusing to save a course without an owner")
	}

	source, err := json.Marshal(course.Source)
	if err != nil {
		return fmt.Errorf("cannot encode course %s: %w", course.Name, err)
	}

	err = db.queries.SaveCourse(ctx, sqlc.SaveCourseParams{
		Owner:      course.Owner,
		Name:       course.Name,
		Source:     source,
		RawYaml:    course.RawYAML,
		ImportedAt: course.ImportedAt,
		UpdatedAt:  course.UpdatedAt,
	})
	if err != nil {
		return fmt.Errorf("cannot save course %s: %w", course.Name, err)
	}
	return nil
}

// DeleteCourse removes a course owned by the given user. Deleting a course that
// does not exist for that owner is ErrCourseNotFound, not a silent success.
func (db *PG) DeleteCourse(ctx context.Context, owner, name string) error {
	rows, err := db.queries.DeleteCourse(ctx, sqlc.DeleteCourseParams{Owner: owner, Name: name})
	if err != nil {
		return fmt.Errorf("cannot delete course %s: %w", name, err)
	}
	if rows == 0 {
		return ErrCourseNotFound
	}
	return nil
}
