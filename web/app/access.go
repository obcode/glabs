package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/obcode/glabs/v3/web/db"
	"github.com/obcode/glabs/v3/web/mail"
	"github.com/obcode/glabs/v3/web/principal"
	"github.com/rs/zerolog/log"
)

// AccessNone is the status of someone who never asked for access: there is no
// row for them. The other states are the db.Access* constants.
const AccessNone = "none"

// maxReasonLen caps the free text of an access request. It is read by a human in
// a mail, so a paragraph is plenty.
const maxReasonLen = 1000

// accessCacheTTL is how long a looked-up status is trusted. The gate asks on
// every GraphQL request; without a cache that is a database round trip per
// request. Every decision made through this process drops the entry at once, so
// the TTL only bounds how stale a status can get from a change made elsewhere
// (by hand in the database).
const accessCacheTTL = 30 * time.Second

// ErrNotApproved is what the access gate returns for any field an unapproved
// user may not use.
var ErrNotApproved = errors.New("not approved: access to glabs must be requested and approved by an administrator")

type accessCacheEntry struct {
	status string
	at     time.Time
}

// accessCache is the per-process status cache. glabs-web runs as one instance, so
// invalidating here is invalidating everywhere.
type accessCache struct {
	mu      sync.Mutex
	entries map[string]accessCacheEntry
}

func (c *accessCache) get(email string, now time.Time) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[email]
	if !ok || now.Sub(e.at) > accessCacheTTL {
		return "", false
	}
	return e.status, true
}

func (c *accessCache) put(email, status string, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[string]accessCacheEntry{}
	}
	c.entries[email] = accessCacheEntry{status: status, at: now}
}

func (c *accessCache) drop(email string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, email)
}

// SetAuthDisabled tells the app that the auth middleware injects the local dev
// user instead of a proxy identity. There is then no one to approve, and every
// request counts as approved. The zero value is "auth enabled", so an App that
// was never told is fail-closed.
func (a *App) SetAuthDisabled(disabled bool) { a.authDisabled = disabled }

// SetPublicURL sets the base URL of the GUI, used for links in access mails.
// Empty means the mails carry no link.
func (a *App) SetPublicURL(u string) { a.publicURL = strings.TrimRight(strings.TrimSpace(u), "/") }

// AccessStatus reports whether email may use glabs: AccessNone or one of the
// db.Access* states. Admins from the config are always approved and need no row,
// which is also what keeps an empty table from locking everyone out.
//
// Except in preview mode: an admin who asked to see glabs as an unapproved user
// gets the status of their own row, like anyone else. That is how an admin can
// walk through the request flow with the one identity the proxy gives them.
func (a *App) AccessStatus(ctx context.Context, email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if a.authDisabled || (a.IsAdminEmail(email) && !previewing(ctx, email)) {
		return db.AccessApproved, nil
	}
	if email == "" {
		return AccessNone, nil
	}
	now := time.Now()
	if status, ok := a.accessCache.get(email, now); ok {
		return status, nil
	}
	row, err := a.db.GetUserAccess(ctx, email)
	if err != nil {
		return "", err
	}
	status := AccessNone
	if row != nil {
		status = row.Status
	}
	a.accessCache.put(email, status, now)
	return status, nil
}

// previewing reports whether the request's own user is email and asked for
// preview mode. Tied to the email so a preview can never affect how anyone else
// is judged.
func previewing(ctx context.Context, email string) bool {
	u := principal.UserFromContext(ctx)
	return u != nil && u.Preview && u.Email == email
}

// IsApproved is AccessStatus reduced to the gate's question, for the request's
// own user. Any error is "no": the gate is fail-closed.
func (a *App) IsApproved(ctx context.Context) bool {
	u := principal.UserFromContext(ctx)
	if u == nil {
		return false
	}
	status, err := a.AccessStatus(ctx, u.Email)
	if err != nil {
		log.Error().Err(err).Str("email", u.Email).Msg("cannot read access status; refusing the request")
		return false
	}
	return status == db.AccessApproved
}

// RequestAccess records that the request's user asks to be let in and mails the
// admins. It is only valid for someone who never asked: a pending request needs
// no second one, and a decided one is reset by an admin, not by asking again.
func (a *App) RequestAccess(ctx context.Context, reason string) error {
	u := principal.UserFromContext(ctx)
	if u == nil {
		return fmt.Errorf("no authenticated user")
	}
	status, err := a.AccessStatus(ctx, u.Email)
	if err != nil {
		return err
	}
	switch status {
	case AccessNone:
	case db.AccessPending:
		return nil // already asked; repeating it is harmless and sends nothing
	case db.AccessApproved:
		return fmt.Errorf("already approved")
	default:
		return fmt.Errorf("access was %s; please contact the administrators", status)
	}

	reason = strings.TrimSpace(reason)
	if len([]rune(reason)) > maxReasonLen {
		return fmt.Errorf("the reason is too long (at most %d characters)", maxReasonLen)
	}
	req := &db.UserAccess{
		Email:       u.Email,
		Name:        strings.TrimSpace(u.Name),
		Department:  strings.TrimSpace(u.Department),
		Status:      db.AccessPending,
		Reason:      reason,
		RequestedAt: time.Now(),
	}
	created, err := a.db.InsertAccessRequest(ctx, req)
	a.accessCache.drop(u.Email)
	if err != nil {
		return err
	}
	if !created {
		return nil // lost a race against the same person's other tab
	}

	a.recordEvent(ctx, &db.Event{
		Type: db.EventAccessRequested, Actor: req.Email, ActorName: req.Name,
		Department: req.Department, Severity: db.SeverityWarning, Detail: "Freischaltung angefragt",
	})

	link := ""
	if a.publicURL != "" {
		link = a.publicURL + "/admin/access?user=" + url.QueryEscape(req.Email)
	}
	data := accessMailData(req, link)
	subject := "glabs: Anfrage auf Freischaltung von " + req.Email
	for admin := range a.admins {
		a.sendAccessMail(admin, subject, mail.TmplAccessRequested, data)
	}
	return nil
}

// UserAccessList returns every access row, open requests first. Admin-only.
func (a *App) UserAccessList(ctx context.Context) ([]*db.UserAccess, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	return a.db.ListUserAccess(ctx)
}

// ApproveUser lets email in and tells them by mail. Admin-only.
func (a *App) ApproveUser(ctx context.Context, email string) (*db.UserAccess, error) {
	return a.decide(ctx, email, db.AccessApproved)
}

// RejectUser turns a request down and tells the requester by mail. Admin-only.
func (a *App) RejectUser(ctx context.Context, email string) (*db.UserAccess, error) {
	return a.decide(ctx, email, db.AccessRejected)
}

// RevokeUser withdraws access. No mail: this is an operator's action, not an
// answer to a request. Admin-only.
func (a *App) RevokeUser(ctx context.Context, email string) (*db.UserAccess, error) {
	return a.decide(ctx, email, db.AccessRevoked)
}

// ResetUser forgets any request or decision for email, so they can ask again.
// Admin-only.
func (a *App) ResetUser(ctx context.Context, email string) (bool, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return false, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	existed, err := a.db.DeleteUserAccess(ctx, email)
	a.accessCache.drop(email)
	if err != nil {
		return false, err
	}
	if existed {
		a.recordEvent(ctx, &db.Event{
			Type: db.EventAccessReset, Actor: principal.UserFromContext(ctx).Email,
			Severity: db.SeverityInfo, Detail: "Freischaltung zurückgesetzt: " + email,
		})
	}
	return existed, nil
}

// decide sets the status of an existing row. Deciding about someone who never
// asked is refused: approving has to start from a request, so there always is
// one to look at.
func (a *App) decide(ctx context.Context, email, status string) (*db.UserAccess, error) {
	if err := a.requireAdmin(ctx); err != nil {
		return nil, err
	}
	admin := principal.UserFromContext(ctx).Email
	email = strings.ToLower(strings.TrimSpace(email))
	row, err := a.db.SetUserAccessStatus(ctx, email, status, admin, time.Now())
	a.accessCache.drop(email)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, fmt.Errorf("no access request from %s", email)
	}

	var (
		eventType, detail, tmpl, subject, link string
		severity                               = db.SeverityInfo
	)
	switch status {
	case db.AccessApproved:
		eventType, detail = db.EventAccessGranted, "freigeschaltet: "+email
		tmpl, subject, link = mail.TmplAccessGranted, "glabs: Du bist freigeschaltet", a.publicURL
	case db.AccessRejected:
		eventType, detail = db.EventAccessRejected, "abgelehnt: "+email
		tmpl, subject = mail.TmplAccessRejected, "glabs: Deine Anfrage auf Freischaltung"
	case db.AccessRevoked:
		eventType, detail, severity = db.EventAccessRevoked, "entzogen: "+email, db.SeverityWarning
	}
	a.recordEvent(ctx, &db.Event{Type: eventType, Actor: admin, Severity: severity, Detail: detail})
	if tmpl != "" {
		a.sendAccessMail(row.Email, subject, tmpl, accessMailData(row, link))
	}
	return row, nil
}

// accessMailData prepares a row for the access templates. Everything in it ends
// up in an HTML mail via Markdown, which passes raw HTML through, so the values
// that do not come from an admin are defused here.
func accessMailData(u *db.UserAccess, link string) mail.AccessMail {
	return mail.AccessMail{
		Email:       u.Email,
		Name:        stripAngles(u.Name),
		Department:  stripAngles(u.Department),
		Reason:      strings.ReplaceAll(u.Reason, "`", "'"),
		RequestedAt: u.RequestedAt,
		Link:        link,
	}
}

func stripAngles(s string) string {
	return strings.NewReplacer("<", "", ">", "").Replace(s)
}

// sendAccessMail renders and sends one access mail. Best-effort, like the job
// mails: the decision is stored whether or not the mail goes out, and the admin
// page shows it either way.
func (a *App) sendAccessMail(to, subject, tmpl string, data mail.AccessMail) {
	if a.mailer == nil {
		log.Warn().Str("to", to).Str("template", tmpl).Msg("no SMTP configured; access mail not sent")
		return
	}
	text, html, err := mail.Render(tmpl, data)
	if err != nil {
		log.Error().Err(err).Str("template", tmpl).Msg("cannot render access mail")
		return
	}
	if err := a.mailer.Send(a.mailDryRun, to, subject, text, html); err != nil {
		log.Error().Err(err).Str("to", to).Str("template", tmpl).Msg("cannot send access mail")
	}
}
