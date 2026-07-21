package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/obcode/glabs/v3/web/db"
)

// GitLabTokenStatus reports whether the caller has stored a GitLab token and when
// — never the token itself.
type GitLabTokenStatus struct {
	Set       bool
	UpdatedAt *time.Time
}

// SetGitLabToken stores the caller's GitLab PAT, AES-256-GCM encrypted. Write-only:
// the token is never returned by any query, and the plaintext is never persisted or
// logged. Fails closed if no secrets.key is configured.
func (a *App) SetGitLabToken(ctx context.Context, token string) (*GitLabTokenStatus, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("token must not be empty")
	}
	if a.sealer == nil {
		return nil, fmt.Errorf("secret storage is unavailable: set secrets.key in the server config")
	}

	sealed, err := a.sealer.Seal(token)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := a.db.SaveUserGitLabToken(ctx, o, sealed, now); err != nil {
		return nil, err
	}
	a.recordEvent(ctx, &db.Event{Type: db.EventTokenSaved, Actor: o, Severity: db.SeverityInfo})
	return &GitLabTokenStatus{Set: true, UpdatedAt: &now}, nil
}

// RemoveGitLabToken deletes the caller's stored GitLab PAT.
func (a *App) RemoveGitLabToken(ctx context.Context) (*GitLabTokenStatus, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.db.DeleteUserGitLabToken(ctx, o); err != nil {
		return nil, err
	}
	a.recordEvent(ctx, &db.Event{Type: db.EventTokenDeleted, Actor: o, Severity: db.SeverityInfo})
	return &GitLabTokenStatus{Set: false}, nil
}

// GitLabTokenStatus reports whether the caller has a stored token, without
// decrypting or returning it.
func (a *App) GitLabTokenStatus(ctx context.Context) (*GitLabTokenStatus, error) {
	o, err := owner(ctx)
	if err != nil {
		return nil, err
	}
	sec, err := a.db.GetUserSecret(ctx, o)
	if err != nil {
		return nil, err
	}
	if sec == nil || sec.GitLab == nil {
		return &GitLabTokenStatus{Set: false}, nil
	}
	return &GitLabTokenStatus{Set: true, UpdatedAt: sec.GitLabUpdatedAt}, nil
}

// Decrypting the stored token for server-side use (building a GitLab client) is
// deliberately not here yet — it arrives with the read-only GitLab operations
// that need it, so the plaintext has exactly one consumer and one code path.
