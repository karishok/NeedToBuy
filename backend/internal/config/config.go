// Package config loads process-wide settings from environment variables.
package config

import "os"

// Config holds settings for the running process.
type Config struct {
	DatabaseURL string
	Port        string
	SMTPAddr    string // host:port of the SMTP relay used to send OTP mail
	SMTPFrom    string // From address on OTP mail
	OTPPepper   string // secret mixed into the OTP code hash
}

// Load reads Config from environment variables, falling back to
// local-development defaults for anything unset.
func Load() Config {
	return Config{
		DatabaseURL: getenv("DATABASE_URL", "postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable"),
		Port:        getenv("PORT", "8080"),
		SMTPAddr:    getenv("SMTP_ADDR", "localhost:1025"),
		SMTPFrom:    getenv("SMTP_FROM", "no-reply@needtobuy.local"),
		OTPPepper:   getenv("OTP_PEPPER", "dev-insecure-pepper-change-me"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
