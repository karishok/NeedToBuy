package child

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const maxNameLength = 100

// validateName trims name and rejects it if it's empty or too long.
func validateName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", errors.New("name is required")
	}
	if utf8.RuneCountInString(trimmed) > maxNameLength {
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
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if d.After(today) {
		return time.Time{}, errors.New("birth_date cannot be in the future")
	}
	return d, nil
}
