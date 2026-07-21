package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/principal"
	"github.com/rs/zerolog/log"
)

// loginThrottle is how long a user's activity counts as one "login". X-Remote-User
// arrives on every request, so without this a busy user would fill the log with an
// event per click; the digest only wants "who was active", so once per window is
// enough.
const loginThrottle = 8 * time.Hour

// platformEventLimit caps the admin page's live feed — a bounded recent view, not
// a full dump (the digest reads its own exact window separately).
const platformEventLimit = 2000

// recordEvent appends one monitoring event. Best-effort by design: a logging
// failure must never fail the thing that happened, so it is logged and swallowed —
// exactly like recordOp for the owner-scoped activity log.
func (a *App) recordEvent(ctx context.Context, e *db.Event) {
	if e.At.IsZero() {
		e.At = time.Now()
	}
	if err := a.db.RecordEvent(ctx, e); err != nil {
		log.Warn().Err(err).Str("type", e.Type).Msg("cannot record monitoring event")
	}
}

// IsAdminEmail reports whether the given email is on the config admins list.
func (a *App) IsAdminEmail(email string) bool {
	return a.admins[strings.ToLower(strings.TrimSpace(email))]
}

// IsAdmin reports whether the request's user is a platform admin (on the config
// admins list). Admins see the monitoring page and receive the nightly summary;
// there is no other privilege — ordinary data access stays strictly owner-scoped.
func (a *App) IsAdmin(ctx context.Context) bool {
	u := principal.UserFromContext(ctx)
	if u == nil {
		return false
	}
	return a.IsAdminEmail(u.Email)
}

// requireAdmin gates the platform-wide reads: unlike every other query these are
// NOT owner-scoped, so they must be refused for anyone not on the admins list.
func (a *App) requireAdmin(ctx context.Context) error {
	if !a.IsAdmin(ctx) {
		return fmt.Errorf("not authorized: this view is for administrators only")
	}
	return nil
}

// PlatformEvents returns the most recent events since `since`, newest first —
// the admin page's live feed. Admin-only.
func (a *App) PlatformEvents(ctx context.Context, since time.Time) ([]*db.Event, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	return a.db.RecentEvents(ctx, since, platformEventLimit)
}

// NoteLogin records that a user was active, throttled to at most one event per
// loginThrottle so the log stays a "who was here" list rather than a request
// trace. Called from the auth middleware for every allowed request; the throttle
// state is in-memory (per process), so a restart re-arms it — acceptable for a
// monitoring signal.
func (a *App) NoteLogin(ctx context.Context, email, name, department string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return
	}
	now := time.Now()
	a.loginMu.Lock()
	if last, seen := a.loginSeen[email]; seen && now.Sub(last) < loginThrottle {
		a.loginMu.Unlock()
		return
	}
	a.loginSeen[email] = now
	a.loginMu.Unlock()

	a.recordEvent(ctx, &db.Event{
		Type:       db.EventLogin,
		Actor:      email,
		ActorName:  strings.TrimSpace(name),
		Department: strings.TrimSpace(department),
		Severity:   db.SeverityInfo,
		Detail:     "angemeldet",
	})
}

// NoteRejectedLogin records a refused request: no identity at all, or an email not
// on the allowlist. These are rare and security-relevant, so each is logged (no
// throttle) as a warning.
func (a *App) NoteRejectedLogin(ctx context.Context, email, department, reason string) {
	a.recordEvent(ctx, &db.Event{
		Type:       db.EventLoginRejected,
		Actor:      strings.ToLower(strings.TrimSpace(email)),
		Department: strings.TrimSpace(department),
		Severity:   db.SeverityWarning,
		Detail:     reason,
	})
}
