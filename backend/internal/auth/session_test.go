package auth

import (
	"context"
	"testing"
	"time"

	"needtobuy/internal/dbtest"
)

func TestUpsertUser_NewEmail_CreatesRow(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	id, err := upsertUser(ctx, tx, "parent@example.com")
	if err != nil {
		t.Fatalf("upsertUser() error = %v", err)
	}
	if id == 0 {
		t.Fatal("upsertUser() returned id = 0, want non-zero")
	}
}

func TestUpsertUser_ExistingEmail_ReturnsSameID(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	first, err := upsertUser(ctx, tx, "parent@example.com")
	if err != nil {
		t.Fatalf("first upsertUser() error = %v", err)
	}
	second, err := upsertUser(ctx, tx, "parent@example.com")
	if err != nil {
		t.Fatalf("second upsertUser() error = %v", err)
	}
	if first != second {
		t.Fatalf("upsertUser() ids = %d, %d, want equal", first, second)
	}
}

func TestCreateSession_ThenLookupSession_ReturnsUserID(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	userID, err := upsertUser(ctx, tx, "parent@example.com")
	if err != nil {
		t.Fatalf("upsertUser() error = %v", err)
	}

	sessionID, err := createSession(ctx, tx, userID)
	if err != nil {
		t.Fatalf("createSession() error = %v", err)
	}
	if sessionID == "" {
		t.Fatal("createSession() returned empty id")
	}

	gotUserID, ok, err := lookupSession(ctx, tx, sessionID)
	if err != nil {
		t.Fatalf("lookupSession() error = %v", err)
	}
	if !ok {
		t.Fatal("lookupSession() ok = false, want true")
	}
	if gotUserID != userID {
		t.Fatalf("lookupSession() userID = %d, want %d", gotUserID, userID)
	}
}

func TestLookupSession_Unknown_ReturnsNotOK(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	_, ok, err := lookupSession(ctx, tx, "does-not-exist")
	if err != nil {
		t.Fatalf("lookupSession() error = %v", err)
	}
	if ok {
		t.Fatal("lookupSession() ok = true, want false")
	}
}

func TestLookupSession_Expired_ReturnsNotOK(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	userID, err := upsertUser(ctx, tx, "parent@example.com")
	if err != nil {
		t.Fatalf("upsertUser() error = %v", err)
	}
	sessionID, err := createSession(ctx, tx, userID)
	if err != nil {
		t.Fatalf("createSession() error = %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE sessions SET expires_at = now() - interval '1 minute' WHERE id = $1`, sessionID); err != nil {
		t.Fatalf("expire session: %v", err)
	}

	_, ok, err := lookupSession(ctx, tx, sessionID)
	if err != nil {
		t.Fatalf("lookupSession() error = %v", err)
	}
	if ok {
		t.Fatal("lookupSession() ok = true, want false for an expired session")
	}
}

func TestLookupSession_ExtendsExpiry(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	userID, err := upsertUser(ctx, tx, "parent@example.com")
	if err != nil {
		t.Fatalf("upsertUser() error = %v", err)
	}
	sessionID, err := createSession(ctx, tx, userID)
	if err != nil {
		t.Fatalf("createSession() error = %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE sessions SET expires_at = now() + interval '1 minute' WHERE id = $1`, sessionID); err != nil {
		t.Fatalf("shorten expiry: %v", err)
	}

	if _, _, err := lookupSession(ctx, tx, sessionID); err != nil {
		t.Fatalf("lookupSession() error = %v", err)
	}

	var expiresAt time.Time
	if err := tx.GetContext(ctx, &expiresAt, `SELECT expires_at FROM sessions WHERE id = $1`, sessionID); err != nil {
		t.Fatalf("read expires_at: %v", err)
	}
	if time.Until(expiresAt) < 29*24*time.Hour {
		t.Fatalf("expires_at = %v, want extended close to 30 days from now", expiresAt)
	}
}

func TestDeleteSession_LookupThenFails(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	userID, err := upsertUser(ctx, tx, "parent@example.com")
	if err != nil {
		t.Fatalf("upsertUser() error = %v", err)
	}
	sessionID, err := createSession(ctx, tx, userID)
	if err != nil {
		t.Fatalf("createSession() error = %v", err)
	}

	if err := deleteSession(ctx, tx, sessionID); err != nil {
		t.Fatalf("deleteSession() error = %v", err)
	}

	_, ok, err := lookupSession(ctx, tx, sessionID)
	if err != nil {
		t.Fatalf("lookupSession() error = %v", err)
	}
	if ok {
		t.Fatal("lookupSession() ok = true after delete, want false")
	}
}

func TestUserID_FromContext(t *testing.T) {
	if _, ok := UserID(context.Background()); ok {
		t.Fatal("UserID() ok = true on a bare context, want false")
	}

	ctx := context.WithValue(context.Background(), userIDContextKey, int64(42))
	id, ok := UserID(ctx)
	if !ok {
		t.Fatal("UserID() ok = false, want true")
	}
	if id != 42 {
		t.Fatalf("UserID() = %d, want 42", id)
	}
}
