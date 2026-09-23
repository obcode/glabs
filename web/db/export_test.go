package db

import "github.com/jackc/pgx/v5/pgxpool"

// PoolForTest exposes the connection pool to the tests in package db_test, which
// need raw SQL to assert things about the SCHEMA itself -- constraints, what a
// column actually contains -- rather than about the methods built on it.
//
// It lives in an _test.go file, so it exists only while testing and never
// becomes part of the package's API.
func (db *PG) PoolForTest() *pgxpool.Pool {
	return db.pool
}
