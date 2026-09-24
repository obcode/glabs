package db

import "time"

// Access states of a row in the users table. "Never asked" has no row and no
// constant here; the app calls it AccessNone.
const (
	AccessPending  = "pending"
	AccessApproved = "approved"
	AccessRejected = "rejected"
	AccessRevoked  = "revoked"
)

// UserAccess is one person who asked to use glabs-web, and what an admin decided.
// PostgreSQL only: there is no MongoDB implementation, as with ReapExpired.
type UserAccess struct {
	Email       string
	Name        string
	Department  string
	Status      string
	Reason      string
	RequestedAt time.Time
	DecidedAt   *time.Time
	DecidedBy   string
}
