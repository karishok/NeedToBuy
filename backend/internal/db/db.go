// Package db provides the Postgres connection and migration tooling
// shared by every domain package.
package db

import (
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// Connect opens a connection pool to Postgres at dsn and verifies
// connectivity with a ping.
func Connect(dsn string) (*sqlx.DB, error) {
	conn, err := sqlx.Connect("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: connect: %w", err)
	}
	return conn, nil
}
