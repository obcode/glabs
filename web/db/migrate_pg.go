package db

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Migrations travel inside the binary, so a release carries its own schema and
// the two can never drift apart across a deploy. The release image is a scratch
// container with nothing in it but the executable -- no goose, no psql -- so
// nobody else *can* run them: whoever deploys a tag applies its schema by
// starting it.
//
// It is also what lets a test migrate a throwaway database without knowing where
// the repository root is.
//
//go:embed migrations/*.sql
var pgMigrationsFS embed.FS

// There are deliberately no down migrations in practice, only the `-- +goose
// Down` sections that make each file reversible on paper. Rolling back means
// deploying the previous image (`deploy.sh web <older-tag>`), and the rule that
// makes that safe is that every migration must be compatible with the PREVIOUS
// binary: add a column, deploy, use it, remove the old one a release later. With
// one host, one instance and a pg_dump before every schema step, that is more
// honest than down scripts nobody ever runs.

// MigrationsChecksum identifies the schema this binary carries. Two binaries
// with the same checksum expect the same tables.
//
// internal/pgtest names its template database after it, so that editing a
// migration produces a fresh template instead of silently reusing a stale one --
// the failure mode that otherwise costs an afternoon of confusing test results.
func MigrationsChecksum() (string, error) {
	entries, err := fs.ReadDir(pgMigrationsFS, "migrations")
	if err != nil {
		return "", fmt.Errorf("cannot read migrations: %w", err)
	}

	sum := sha256.New()
	for _, entry := range entries {
		content, err := fs.ReadFile(pgMigrationsFS, "migrations/"+entry.Name())
		if err != nil {
			return "", fmt.Errorf("cannot read %s: %w", entry.Name(), err)
		}
		// The name matters as much as the content: a rename reorders the run.
		sum.Write([]byte(entry.Name()))
		sum.Write(content)
	}

	return hex.EncodeToString(sum.Sum(nil))[:12], nil
}

// MigratePG brings the database up to the schema this binary was built with.
//
// Fatal on failure, unlike the EnsureXIndexes calls it replaces: a half-migrated
// schema means the queries compiled into this binary do not match the database,
// and every later error would be a confusing symptom of that one cause.
//
// Repeating it is a no-op -- goose applies only what is missing.
func MigratePG(ctx context.Context, pool *pgxpool.Pool) error {
	goose.SetBaseFS(pgMigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("cannot set goose dialect: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close() //nolint:errcheck

	if err := goose.UpContext(ctx, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("cannot migrate database: %w", err)
	}

	return nil
}

// MigrateSchema brings the connected database up to the schema this binary
// carries. It is what the server calls at startup.
func (db *PG) MigrateSchema(ctx context.Context) error {
	return MigratePG(ctx, db.pool)
}
