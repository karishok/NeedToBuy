package config_test

import (
	"testing"

	"needtobuy/internal/config"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORT", "")
	t.Setenv("SMTP_ADDR", "")
	t.Setenv("SMTP_FROM", "")
	t.Setenv("OTP_PEPPER", "")

	cfg := config.Load()

	want := "postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable"
	if cfg.DatabaseURL != want {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.SMTPAddr != "localhost:1025" {
		t.Errorf("SMTPAddr = %q, want %q", cfg.SMTPAddr, "localhost:1025")
	}
	if cfg.SMTPFrom != "no-reply@needtobuy.local" {
		t.Errorf("SMTPFrom = %q, want %q", cfg.SMTPFrom, "no-reply@needtobuy.local")
	}
	if cfg.OTPPepper == "" {
		t.Error("OTPPepper = \"\", want a non-empty default")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://custom/db")
	t.Setenv("PORT", "9090")
	t.Setenv("SMTP_ADDR", "smtp.example.com:587")
	t.Setenv("SMTP_FROM", "hello@needtobuy.ru")
	t.Setenv("OTP_PEPPER", "prod-secret")

	cfg := config.Load()

	if cfg.DatabaseURL != "postgres://custom/db" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://custom/db")
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.SMTPAddr != "smtp.example.com:587" {
		t.Errorf("SMTPAddr = %q, want %q", cfg.SMTPAddr, "smtp.example.com:587")
	}
	if cfg.SMTPFrom != "hello@needtobuy.ru" {
		t.Errorf("SMTPFrom = %q, want %q", cfg.SMTPFrom, "hello@needtobuy.ru")
	}
	if cfg.OTPPepper != "prod-secret" {
		t.Errorf("OTPPepper = %q, want %q", cfg.OTPPepper, "prod-secret")
	}
}
