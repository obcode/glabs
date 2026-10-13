package db

import "time"

// Event types. Unlike the owner-scoped activity log (which records only the six
// mutating GitLab ops of a single user), the event log is the platform-wide
// monitoring trail an operator reads: who signed in, which jobs were scheduled and
// how they ended, and anything else worth watching. It is deliberately NOT
// owner-scoped — an admin reads across all users.
const (
	EventLogin         = "login"          // a user was active (throttled: at most once per window)
	EventLoginRejected = "login-rejected" // a request without an identity was refused
	EventJobScheduled  = "job-scheduled"  // an operation was queued to run later
	EventJobDone       = "job-done"
	EventJobFailed     = "job-failed"
	EventJobExpired    = "job-expired"
	EventJobCancelled  = "job-cancelled"
	EventOpDone        = "op-done" // an interactive (run-now) operation finished
	EventOpFailed      = "op-failed"
	EventCourseCreated = "course-created"
	EventCourseDeleted = "course-deleted"
	EventTokenSaved    = "token-saved"
	EventTokenDeleted  = "token-deleted"

	EventAccessRequested = "access-requested" // someone asked to be let in
	EventAccessGranted   = "access-granted"   // an admin approved a request
	EventAccessRejected  = "access-rejected"  // an admin turned a request down
	EventAccessRevoked   = "access-revoked"   // an admin withdrew an approval
	EventAccessReset     = "access-reset"     // an admin cleared a decision; the person may ask again
)

// Event severities, used to highlight what matters in the digest and the admin
// page. They live here (not in config) to stay independent of the linter's
// Severity type.
const (
	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityError   = "error"
)

// eventRetention keeps events for 180 days: the monitoring trail is a rolling
// window, not a permanent archive. Enforced by PG.ReapExpired.
const eventRetention = 180 * 24 * time.Hour

// Event is one thing that happened on the platform, worth an operator's
// attention. Most fields are optional and depend on the type: a login carries an
// actor (and maybe a department) but no course; a job event carries course,
// assignment and op. Ownership is NOT enforced — this table exists precisely to
// be read across all users by an admin.
type Event struct {
	At   time.Time
	Type string
	// Actor is the acting user's email (lowercased); empty for an anonymous
	// rejected login (no identity header at all).
	Actor      string
	ActorName  string
	Department string
	Course     string
	Assignment string
	Op         string
	Severity   string
	Detail     string
	JobID      string
}
