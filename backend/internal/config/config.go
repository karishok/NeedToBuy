// Package config loads process-wide settings from environment variables.
package config

import "os"

// Config holds settings for the running process.
type Config struct {
	DatabaseURL string
	Port        string
}

// Load reads Config from environment variables, falling back to
// local-development defaults for anything unset.
func Load() Config {
	return Config{
		DatabaseURL: getenv("DATABASE_URL", "postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable"),
		Port:        getenv("PORT", "8080"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
