package child

import (
	"errors"
	"strings"
	"time"
)

const maxNameLength = 100

// validateName trims name and rejects it if it's empty or too long.
func validateName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("name is required")
	}
	if len(trimmed) > maxNameLength {
		return "", errors.New("name must be at most 100 characters")
	}
	return trimmed, nil
}

// parseBirthDate parses a YYYY-MM-DD date string and rejects it if it's
// malformed or in the future.
func parseBirthDate(s string) (time.Time, error) {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, errors.New("birth_date must be a valid date (YYYY-MM-DD)")
	}
	if d.After(time.Now()) {
		return time.Time{}, errors.New("birth_date cannot be in the future")
	}
	return d, nil
}
