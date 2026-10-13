package db

import "time"

// systemStateID is the fixed id of the single system-state row. There is exactly
// one, holding server-wide bookkeeping that is not tied to any user.
const systemStateID = "state"

// SystemState holds server-wide bookkeeping. Today it records only when the last
// nightly summary was sent, so the digest window survives restarts and a summary
// is never sent twice for the same period.
type SystemState struct {
	ID            string
	SummarySentAt *time.Time
}
