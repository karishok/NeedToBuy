package db_test

import (
	"database/sql"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
)

func TestMigrate_CreatesUsersTable(t *testing.T) {
	dsn := dbtest.DSN(t)

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer conn.Close()

	var tableName string
	if err := conn.QueryRow("SELECT to_regclass('public.users')::text").Scan(&tableName); err != nil {
		t.Fatalf("query error = %v", err)
	}
	if tableName != "users" {
		t.Fatalf("expected users table to exist, got %q", tableName)
	}
}

func TestMigrate_IsIdempotent(t *testing.T) {
	dsn := dbtest.DSN(t)

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
}
