package auth

import (
	"context"
	"fmt"
)

// upsertUser returns the id of the users row for email, creating one if it
// doesn't exist yet. There is no separate sign-up flow for parents — the
// first successful OTP verify for an email is the registration.
func upsertUser(ctx context.Context, db Querier, email string) (int64, error) {
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

// emailByUserID returns the email for a user id, used by GET /api/auth/me
// to report who's logged in.
func emailByUserID(ctx context.Context, db Querier, userID int64) (string, error) {
	var email string
	if err := db.GetContext(ctx, &email, `SELECT email FROM users WHERE id = $1`, userID); err != nil {
		return "", fmt.Errorf("auth: lookup email for user %d: %w", userID, err)
	}
	return email, nil
}
