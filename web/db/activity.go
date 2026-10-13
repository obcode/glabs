package db

import "time"

// activityLimit caps how many log entries a single read returns — the GUI shows
// recent history per assignment, not an unbounded audit trail.
const activityLimit = 200

// ActivityEntry records one mutating operation performed through the web against
// an assignment: what ran, when, and how it ended. It is the web's stand-in for
// the shell history the CLI leaves behind — the course page reads it to show,
// per assignment, what has already been done (setaccess, protect, archive,
// delete; later generate). Ownership is strict, exactly like courses: an entry
// belongs to the user who caused it and no other user can see it.
type ActivityEntry struct {
	Owner      string
	Course     string
	Assignment string
	Op         string
	Params     map[string]string
	// Status is the terminal outcome — "done" or "failed".
	Status string
	// Detail is a short human summary: the repository count on success, the error
	// message on failure.
	Detail string
	At     time.Time
}
