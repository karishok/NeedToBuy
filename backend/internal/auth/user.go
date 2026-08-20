package auth

import (
	"context"
	"fmt"
)

// upsertUser returns the id of the users row for email, creating one if it
// doesn't exist yet. There is no separate sign-up flow for parents — the
// first successful OTP verify for an email is the registration.
func upsertUser(ctx context.Context, db querier, email string) (int64, error) {
	var id int64
	err := db.GetContext(ctx, &id, `
		INSERT INTO users (email) VALUES ($1)
		ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email
		RETURNING id`, email)
	if err != nil {
		return 0, fmt.Errorf("auth: upsert user: %w", err)
	}
	return id, nil
}
