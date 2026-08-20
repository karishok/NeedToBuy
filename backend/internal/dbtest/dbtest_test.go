package dbtest_test

import (
	"context"
	"testing"

	"needtobuy/internal/dbtest"
)

func TestTx_RollsBackBetweenCalls(t *testing.T) {
	first := dbtest.Tx(t)
	if _, err := first.ExecContext(context.Background(),
		`INSERT INTO users (email) VALUES ($1)`, "dbtest-rollback@example.com"); err != nil {
		t.Fatalf("insert in first tx: %v", err)
	}

	var count int
	if err := first.GetContext(context.Background(), &count,
		`SELECT count(*) FROM users WHERE email = $1`, "dbtest-rollback@example.com"); err != nil {
		t.Fatalf("count in first tx: %v", err)
	}
	if count != 1 {
		t.Fatalf("count in first tx = %d, want 1", count)
	}

	second := dbtest.Tx(t)
	if err := second.GetContext(context.Background(), &count,
		`SELECT count(*) FROM users WHERE email = $1`, "dbtest-rollback@example.com"); err != nil {
		t.Fatalf("count in second tx: %v", err)
	}
	if count != 0 {
		t.Fatalf("count in second tx = %d, want 0 (first tx should have rolled back)", count)
	}
}
