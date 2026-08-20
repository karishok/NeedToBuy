// Package dbtest provides shared helpers for tests that need a real
// Postgres connection (started via `docker compose up -d`).
package dbtest

import (
	"os"
	"testing"
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
