package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"time"
)

const (
	otpCodeTTL        = 10 * time.Minute
	otpMaxAttempts    = 5
	otpResendCooldown = 60 * time.Second
)

// errTooSoon signals the resend cooldown for this email hasn't elapsed yet.
var errTooSoon = errors.New("auth: resend cooldown active")

// errInvalidCode signals the submitted code is wrong, expired, or has
// already burned through its attempt budget — callers should not
// distinguish between these cases in the API response.
var errInvalidCode = errors.New("auth: invalid or expired code")

// otpCode mirrors one row of otp_codes.
type otpCode struct {
	ID         int64      `db:"id"`
	Email      string     `db:"email"`
	CodeHash   string     `db:"code_hash"`
	Attempts   int        `db:"attempts"`
	ExpiresAt  time.Time  `db:"expires_at"`
	ConsumedAt *time.Time `db:"consumed_at"`
	CreatedAt  time.Time  `db:"created_at"`
}

// generateCode returns a random 6-digit numeric OTP code.
func generateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("auth: generate code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// hashCode derives the value stored in otp_codes.code_hash.
func hashCode(code, email, pepper string) string {
	sum := sha256.Sum256([]byte(code + email + pepper))
	return hex.EncodeToString(sum[:])
}

// createOTP enforces the resend cooldown, then inserts a new OTP code row
// for email and returns the plaintext code to send by mail.
func createOTP(ctx context.Context, db querier, pepper, email string) (string, error) {
	var last otpCode
	err := db.GetContext(ctx, &last, `
		SELECT id, email, code_hash, attempts, expires_at, consumed_at, created_at
		FROM otp_codes WHERE email = $1 ORDER BY created_at DESC LIMIT 1`, email)
	if err == nil && time.Since(last.CreatedAt) < otpResendCooldown {
		return "", errTooSoon
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("auth: lookup last otp: %w", err)
	}

	code, err := generateCode()
	if err != nil {
		return "", err
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO otp_codes (email, code_hash, expires_at)
		VALUES ($1, $2, $3)`, email, hashCode(code, email, pepper), time.Now().Add(otpCodeTTL)); err != nil {
		return "", fmt.Errorf("auth: insert otp: %w", err)
	}
	return code, nil
}

// verifyOTP checks code against the most recent unconsumed OTP for email.
// A wrong code increments the row's attempt counter; once attempts reach
// otpMaxAttempts, or the code has expired, every subsequent call returns
// errInvalidCode regardless of what code is submitted. On success the code
// is marked consumed so it cannot be replayed.
func verifyOTP(ctx context.Context, db querier, pepper, email, code string) error {
	var row otpCode
	err := db.GetContext(ctx, &row, `
		SELECT id, email, code_hash, attempts, expires_at, consumed_at, created_at
		FROM otp_codes
		WHERE email = $1 AND consumed_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, email)
	if errors.Is(err, sql.ErrNoRows) {
		return errInvalidCode
	}
	if err != nil {
		return fmt.Errorf("auth: lookup otp: %w", err)
	}

	if row.Attempts >= otpMaxAttempts || time.Now().After(row.ExpiresAt) {
		return errInvalidCode
	}

	if row.CodeHash != hashCode(code, email, pepper) {
		if _, err := db.ExecContext(ctx,
			`UPDATE otp_codes SET attempts = attempts + 1 WHERE id = $1`, row.ID); err != nil {
			return fmt.Errorf("auth: record failed attempt: %w", err)
		}
		return errInvalidCode
	}

	if _, err := db.ExecContext(ctx,
		`UPDATE otp_codes SET consumed_at = now() WHERE id = $1`, row.ID); err != nil {
		return fmt.Errorf("auth: consume otp: %w", err)
	}
	return nil
}
