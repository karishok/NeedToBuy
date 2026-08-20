package auth

import (
	"context"
	"errors"
	"testing"

	"needtobuy/internal/dbtest"
)

func TestCreateOTP_ThenVerify_Succeeds(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	code, err := createOTP(ctx, tx, "pepper", "parent@example.com")
	if err != nil {
		t.Fatalf("createOTP() error = %v", err)
	}
	if len(code) != 6 {
		t.Fatalf("code = %q, want 6 digits", code)
	}

	if err := verifyOTP(ctx, tx, "pepper", "parent@example.com", code); err != nil {
		t.Fatalf("verifyOTP() error = %v", err)
	}
}

func TestVerifyOTP_WrongCode_ReturnsErrInvalidCode(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	code, err := createOTP(ctx, tx, "pepper", "parent@example.com")
	if err != nil {
		t.Fatalf("createOTP() error = %v", err)
	}
	wrong := "000000"
	if wrong == code {
		wrong = "000001"
	}

	if err := verifyOTP(ctx, tx, "pepper", "parent@example.com", wrong); !errors.Is(err, errInvalidCode) {
		t.Fatalf("verifyOTP() error = %v, want errInvalidCode", err)
	}
}

func TestVerifyOTP_ConsumedCode_CannotBeReused(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	code, err := createOTP(ctx, tx, "pepper", "parent@example.com")
	if err != nil {
		t.Fatalf("createOTP() error = %v", err)
	}
	if err := verifyOTP(ctx, tx, "pepper", "parent@example.com", code); err != nil {
		t.Fatalf("first verifyOTP() error = %v", err)
	}

	if err := verifyOTP(ctx, tx, "pepper", "parent@example.com", code); !errors.Is(err, errInvalidCode) {
		t.Fatalf("second verifyOTP() error = %v, want errInvalidCode", err)
	}
}

func TestCreateOTP_WithinCooldown_ReturnsErrTooSoon(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	if _, err := createOTP(ctx, tx, "pepper", "parent@example.com"); err != nil {
		t.Fatalf("first createOTP() error = %v", err)
	}

	if _, err := createOTP(ctx, tx, "pepper", "parent@example.com"); !errors.Is(err, errTooSoon) {
		t.Fatalf("second createOTP() error = %v, want errTooSoon", err)
	}
}

func TestVerifyOTP_TooManyAttempts_BurnsEvenCorrectCode(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	code, err := createOTP(ctx, tx, "pepper", "parent@example.com")
	if err != nil {
		t.Fatalf("createOTP() error = %v", err)
	}
	wrong := "000000"
	if wrong == code {
		wrong = "000001"
	}

	for i := 0; i < otpMaxAttempts; i++ {
		if err := verifyOTP(ctx, tx, "pepper", "parent@example.com", wrong); !errors.Is(err, errInvalidCode) {
			t.Fatalf("attempt %d: verifyOTP() error = %v, want errInvalidCode", i, err)
		}
	}

	if err := verifyOTP(ctx, tx, "pepper", "parent@example.com", code); !errors.Is(err, errInvalidCode) {
		t.Fatalf("verifyOTP() with correct code after exhausting attempts error = %v, want errInvalidCode", err)
	}
}

func TestVerifyOTP_Expired_ReturnsErrInvalidCode(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	code, err := createOTP(ctx, tx, "pepper", "parent@example.com")
	if err != nil {
		t.Fatalf("createOTP() error = %v", err)
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE otp_codes SET expires_at = now() - interval '1 minute' WHERE email = $1`,
		"parent@example.com"); err != nil {
		t.Fatalf("expire code: %v", err)
	}

	if err := verifyOTP(ctx, tx, "pepper", "parent@example.com", code); !errors.Is(err, errInvalidCode) {
		t.Fatalf("verifyOTP() error = %v, want errInvalidCode", err)
	}
}
