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

func TestMigrate_CreatesOTPCodesTable(t *testing.T) {
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
	if err := conn.QueryRow("SELECT to_regclass('public.otp_codes')::text").Scan(&tableName); err != nil {
		t.Fatalf("query error = %v", err)
	}
	if tableName != "otp_codes" {
		t.Fatalf("expected otp_codes table to exist, got %q", tableName)
	}
}

func TestMigrate_CreatesSessionsTable(t *testing.T) {
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
	if err := conn.QueryRow("SELECT to_regclass('public.sessions')::text").Scan(&tableName); err != nil {
		t.Fatalf("query error = %v", err)
	}
	if tableName != "sessions" {
		t.Fatalf("expected sessions table to exist, got %q", tableName)
	}
}

func TestMigrate_CreatesChildrenTable(t *testing.T) {
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
	if err := conn.QueryRow("SELECT to_regclass('public.children')::text").Scan(&tableName); err != nil {
		t.Fatalf("query error = %v", err)
	}
	if tableName != "children" {
		t.Fatalf("expected children table to exist, got %q", tableName)
	}
}
