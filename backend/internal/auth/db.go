// Package auth implements email+OTP login: requesting and verifying a
// one-time code, and the session cookie that keeps a parent logged in
// afterward.
package auth

import (
	"context"
	"database/sql"
)

// querier is satisfied by both *sqlx.DB and *sqlx.Tx, so repository
// functions in this package run unchanged against a live connection in
// production and inside a rollback transaction in tests.
type querier interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
