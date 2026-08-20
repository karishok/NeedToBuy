// Package auth implements email+OTP login: requesting and verifying a
// one-time code, and the session cookie that keeps a parent logged in
// afterward.
package auth

import (
	"context"
	"database/sql"
)

// Querier is satisfied by both *sqlx.DB and *sqlx.Tx, so repository
// functions in this package run unchanged against a live connection in
// production and inside a rollback transaction in tests. It is exported
// so wrapper/decorator packages (logging, metrics) can reference it, e.g.
// to implement it themselves or assert `var _ auth.Querier = someType{}`.
type Querier interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
