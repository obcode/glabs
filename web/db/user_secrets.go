package db

import (
	"time"

	"github.com/obcode/glabs/v3/web/secrets"
)

// UserSecret holds a user's encrypted per-user secrets, keyed by the owner's email
// — here, the GitLab personal access token. The value is AES-256-GCM sealed; the
// plaintext never touches the database. This row is never exposed over GraphQL,
// only a "set / when" status is.
type UserSecret struct {
	Owner           string
	GitLab          *secrets.SealedValue
	GitLabUpdatedAt *time.Time
}
