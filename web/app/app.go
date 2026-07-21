// Package app is glabs-web's core: the layer the GraphQL resolvers delegate to,
// holding the database and enforcing that every request acts only as its own
// user. Resolvers stay thin — auth gate plus a call into here — so the rules
// live in one place rather than scattered across the schema.
package app

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/graph/model"
	"github.com/obcode/glabs/v3/web/secrets"
	"github.com/obcode/glabs/v3/web/zpa"
	"github.com/spf13/viper"
)

// store is the slice of the database the app uses. It is an interface so the
// app's owner-scoping can be tested without a real MongoDB: a fake store records
// which owner each call was given, proving the owner comes from the authenticated
// principal and never from a caller-supplied argument.
type store interface {
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	CoursesOf(ctx context.Context, owner string) ([]*db.StoredCourse, error)
	CourseOf(ctx context.Context, owner, name string) (*db.StoredCourse, error)
	SaveCourse(ctx context.Context, course *db.StoredCourse) error
	DeleteCourse(ctx context.Context, owner, name string) error
	GetUserSecret(ctx context.Context, owner string) (*db.UserSecret, error)
	SaveUserGitLabToken(ctx context.Context, owner string, sealed secrets.SealedValue, updatedAt time.Time) error
	DeleteUserGitLabToken(ctx context.Context, owner string) error
	RecordActivity(ctx context.Context, e *db.ActivityEntry) error
	ActivityFor(ctx context.Context, owner, course, assignment string) ([]*db.ActivityEntry, error)
	CourseActivityFor(ctx context.Context, owner, course string) ([]*db.ActivityEntry, error)
	AllActivityFor(ctx context.Context, owner string) ([]*db.ActivityEntry, error)
	SaveJob(ctx context.Context, job *db.ScheduledJob) error
	CancelJob(ctx context.Context, owner, id string) (*db.ScheduledJob, error)
	JobsOf(ctx context.Context, owner string, statuses []string) ([]*db.ScheduledJob, error)
	JobOf(ctx context.Context, owner, id string) (*db.ScheduledJob, error)
	ClaimDueJob(ctx context.Context, workerID string, now time.Time) (*db.ScheduledJob, error)
	FinishJob(ctx context.Context, id, status, logText, errText string) error
	UnnotifiedTerminalJobs(ctx context.Context) ([]*db.ScheduledJob, error)
	MarkNotified(ctx context.Context, id string) error
	RecordEvent(ctx context.Context, e *db.Event) error
	EventsBetween(ctx context.Context, since, until time.Time) ([]*db.Event, error)
	RecentEvents(ctx context.Context, since time.Time, limit int64) ([]*db.Event, error)
	SystemState(ctx context.Context) (*db.SystemState, error)
	SetSummarySentAt(ctx context.Context, at time.Time) error
}

// Mailer sends a rendered notification. *mail.Sender implements it; the App holds
// one only when SMTP is configured (nil otherwise, so notifications are skipped).
type Mailer interface {
	Send(dryRun bool, to, subject string, text, html []byte) error
}

type App struct {
	db store
	// sealer encrypts per-user secrets at rest. It is nil when no secrets.key is
	// configured, in which case token operations fail closed rather than storing
	// a token in plaintext.
	sealer *secrets.Sealer
	// gitlabHost is the base URL glabs builds project URLs against; it feeds the
	// resolved-config preview so the web shows the same URLs the CLI would.
	gitlabHost string
	// ops serializes mutating operations per (owner, course, assignment).
	ops *opGuard
	// mailer sends job notifications; nil when no SMTP is configured (then
	// notifications are silently skipped). mailDryRun redirects every send to the
	// configured test recipient.
	mailer     Mailer
	mailDryRun bool
	// zpa enriches the roster with student details; nil when ZPA is not configured
	// (then the students page shows just the emails).
	zpa *zpa.Client
	// admins holds the lowercased emails allowed to see the platform-wide monitoring
	// (the admin page and the nightly summary). Empty means no one is an admin.
	admins map[string]bool
	// loginMu/loginSeen throttle login events: X-Remote-User arrives on every
	// request, so a "login" is recorded at most once per user per loginThrottle.
	loginMu   sync.Mutex
	loginSeen map[string]time.Time
	// summaryHour/summaryRecipients configure the nightly digest (set via
	// ConfigureSummary from bootstrap); the scheduler and the "send now" path share
	// them.
	summaryHour       int
	summaryRecipients []string
}

func New(database *db.DB, sealer *secrets.Sealer, gitlabHost string, mailer Mailer, mailDryRun bool, zpaClient *zpa.Client, admins []string) *App {
	adminSet := make(map[string]bool, len(admins))
	for _, e := range admins {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			adminSet[e] = true
		}
	}
	return &App{
		db:         database,
		sealer:     sealer,
		gitlabHost: gitlabHost,
		ops:        newOpGuard(),
		mailer:     mailer,
		mailDryRun: mailDryRun,
		zpa:        zpaClient,
		admins:     adminSet,
		loginSeen:  map[string]time.Time{},
	}
}

// GetUserByEmail looks up a user for the auth middleware's allowlist check.
func (a *App) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	return a.db.GetUserByEmail(ctx, email)
}

// LocalDevUser is the identity used when auth is disabled (local development). It
// is never consulted when auth is enabled.
func (a *App) LocalDevUser() *model.User {
	email := strings.ToLower(strings.TrimSpace(viper.GetString("auth.devuser")))
	if email == "" {
		email = "local@localhost"
	}
	return &model.User{Email: email, Name: "Local Dev User"}
}
