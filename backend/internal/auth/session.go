package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const (
	sessionTTL    = 30 * 24 * time.Hour
	sessionCookie = "session"
)

type contextKey string

const userIDContextKey contextKey = "auth.userID"

// UserID returns the authenticated parent's user id from ctx, and whether
// one was present. Handler.Middleware sets it on every request that
// carries a live session cookie.
func UserID(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(userIDContextKey).(int64)
	return id, ok
}

// generateSessionID returns a random opaque session identifier, used
// directly as the cookie value.
func generateSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: generate session id: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// createSession opens a new session for userID and returns its id.
func createSession(ctx context.Context, db Querier, userID int64) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", err
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, expires_at) VALUES ($1, $2, $3)`,
		id, userID, time.Now().Add(sessionTTL)); err != nil {
		return "", fmt.Errorf("auth: insert session: %w", err)
	}
	return id, nil
}

// lookupSession returns the user id for a live session, sliding its
// expiry forward by sessionTTL. ok is false if the session is missing or
// has expired.
func lookupSession(ctx context.Context, db Querier, sessionID string) (int64, bool, error) {
	var row struct {
		UserID    int64     `db:"user_id"`
		ExpiresAt time.Time `db:"expires_at"`
	}
	err := db.GetContext(ctx, &row, `SELECT user_id, expires_at FROM sessions WHERE id = $1`, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("auth: lookup session: %w", err)
	}
	if time.Now().After(row.ExpiresAt) {
		return 0, false, nil
	}

	if _, err := db.ExecContext(ctx,
		`UPDATE sessions SET expires_at = $1 WHERE id = $2`, time.Now().Add(sessionTTL), sessionID); err != nil {
		return 0, false, fmt.Errorf("auth: extend session: %w", err)
	}
	return row.UserID, true, nil
}

// deleteSession removes a session row (used by logout).
func deleteSession(ctx context.Context, db Querier, sessionID string) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID); err != nil {
		return fmt.Errorf("auth: delete session: %w", err)
	}
	return nil
}

// setSessionCookie stamps the session cookie described in the design doc:
// HttpOnly, Secure, SameSite=Lax, 30-day MaxAge.
func setSessionCookie(w http.ResponseWriter, sessionID string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})
}

// clearSessionCookie expires the session cookie immediately (logout).
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}
