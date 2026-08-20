// Package dbtest provides shared helpers for tests that need a real
// Postgres connection (started via `docker compose up -d`).
package dbtest

import (
	"os"
	"testing"

	"github.com/jmoiron/sqlx"

	"needtobuy/internal/db"
)

// DSN returns the TEST_DATABASE_URL environment variable, skipping the
// calling test if it isn't set.
func DSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; run `docker compose up -d` and export TEST_DATABASE_URL=postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable to run this test")
	}
	return dsn
}

// Tx opens a transaction against a migrated, real Postgres database and
// registers a cleanup that rolls it back, so tests that write rows never
// leak them into later test runs.
func Tx(t *testing.T) *sqlx.Tx {
	t.Helper()
	dsn := DSN(t)

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	tx, err := conn.Beginx()
	if err != nil {
		t.Fatalf("Beginx() error = %v", err)
	}
	t.Cleanup(func() { tx.Rollback() })

	return tx
}
