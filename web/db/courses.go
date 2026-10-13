package db

import (
	"errors"
	"time"

	"github.com/obcode/glabs/v3/config"
)

// StoredCourse is a course as saved by one user. Ownership is strict: a course
// belongs to the user who imported it, and no other user can see or touch it.
//
// RawYAML is kept verbatim alongside the parsed Source so a download can return
// exactly what was uploaded — comments and key order and all — as long as the
// course has not been edited through the web. Re-encoding Source would lose them.
type StoredCourse struct {
	Owner      string
	Name       string
	Source     *config.CourseSource
	RawYAML    []byte
	ImportedAt time.Time
	UpdatedAt  time.Time
}

// ErrCourseNotFound is returned when a course does not exist for the given owner.
// It deliberately does not distinguish "does not exist" from "belongs to someone
// else": to one user, another user's course simply is not there.
var ErrCourseNotFound = errors.New("course not found")

// Every course method on the store takes an owner and filters on it. There is
// deliberately none that reads a course by name alone: owner isolation is
// enforced by the shape of the API, not by remembering to add a filter at each
// call site.
