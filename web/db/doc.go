// Package db is glabs-web's persistence layer. It owns the connection pool and
// the queries; everything above it works with the decoded Go structs.
//
// PostgreSQL (PG) is the only implementation. Until 2026-10-13 there was a second
// one on MongoDB, and web/db/storetest existed to run one suite against both —
// that equivalence is what made the switch a config change rather than a leap of
// faith. The shared types and constants still live in the unsuffixed files
// (courses.go, jobs.go, …) next to their *_pg.go implementations, which is also
// how users.go was always arranged.
package db
