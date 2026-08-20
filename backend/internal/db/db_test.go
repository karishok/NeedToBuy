package db_test

import (
	"testing"

	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
)

func TestConnect_InvalidDSN(t *testing.T) {
	_, err := db.Connect("not-a-valid-dsn")
	if err == nil {
		t.Fatal("expected error for invalid DSN, got nil")
	}
}

func TestConnect_Success(t *testing.T) {
	dsn := dbtest.DSN(t)

	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
}
