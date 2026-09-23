package db

import (
	"context"
	"fmt"
)

// The two methods here exist only for the one-off import from MongoDB
// (web/migrate). They are cross-cutting in a way the rest of this package
// deliberately is not -- nothing else may count or empty the whole database --
// which is why they live in their own file, named for the reason they exist, and
// go away together with the importer in the clean-up step.
//
// Do not build on them.

// ImportTables lists the tables the importer fills, in the order it fills them:
// the irreplaceable first, so that a run which dies half way has already carried
// across the part nobody can recreate.
var ImportTables = []string{
	"system_state",
	"user_secrets",
	"courses",
	"scheduled_jobs",
	"activity",
	"events",
}

// CountsForImport returns the number of rows in each table.
//
// It answers two questions for the importer: is the target empty (so a run
// cannot silently duplicate a previous one), and afterwards, does every table
// hold as many rows as MongoDB did.
func (db *PG) CountsForImport(ctx context.Context) (map[string]int64, error) {
	counts := make(map[string]int64, len(ImportTables))
	for _, table := range ImportTables {
		var n int64
		// The table names come from the constant above, never from input.
		if err := db.pool.QueryRow(ctx, "select count(*) from "+table).Scan(&n); err != nil {
			return nil, fmt.Errorf("cannot count %s: %w", table, err)
		}
		counts[table] = n
	}
	return counts, nil
}

// EnsureSecretsRowForImport creates a secrets row that holds no token.
//
// MongoDB's token deletion unsets the fields and keeps the document, so a user
// who once had a token and removed it still has a row. Functionally the two are
// the same -- every caller checks `s == nil || s.GitLab == nil` -- but a
// migration that silently drops rows is the wrong precedent, and the schema
// comment on user_secrets says the row is deliberately kept. Carrying it across
// also lets the row counts be compared without an exception.
func (db *PG) EnsureSecretsRowForImport(ctx context.Context, owner string) error {
	_, err := db.pool.Exec(ctx,
		"insert into user_secrets (owner) values ($1) on conflict (owner) do nothing", owner)
	if err != nil {
		return fmt.Errorf("cannot create the secrets row of %s: %w", owner, err)
	}
	return nil
}

// TruncateAllForImport empties every table so an aborted import can simply be
// run again.
//
// Rerunning is what makes the cut-over rehearsable, and truncate-then-load is
// the only honest way to get it here: activity and events have no natural key,
// so there is nothing for an upsert to conflict on, and a second run would
// silently double every log entry.
//
// system_state is re-seeded afterwards, because the migration that created it
// only runs once.
func (db *PG) TruncateAllForImport(ctx context.Context) error {
	// One statement: the tables have no foreign keys between them, but a single
	// truncate is still one transaction rather than six.
	stmt := "truncate table"
	for i, table := range ImportTables {
		if i > 0 {
			stmt += ","
		}
		stmt += " " + table
	}
	stmt += " restart identity"

	if _, err := db.pool.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("cannot empty the target tables: %w", err)
	}
	if _, err := db.pool.Exec(ctx,
		"insert into system_state (id) values ('state') on conflict do nothing"); err != nil {
		return fmt.Errorf("cannot re-seed system_state: %w", err)
	}
	return nil
}
