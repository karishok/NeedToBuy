package agerange_test

import (
	"testing"
	"time"

	"needtobuy/internal/agerange"
)

func TestCodeFor(t *testing.T) {
	birth := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name string
		asOf time.Time
		want string
	}{
		{"newborn", birth, "0m"},
		{"just before 1 month", birth.AddDate(0, 1, -1), "0m"},
		{"exactly 1 month", birth.AddDate(0, 1, 0), "1m"},
		{"exactly 6 months", birth.AddDate(0, 6, 0), "6m"},
		{"just before 9 months", birth.AddDate(0, 9, -1), "6m"},
		{"exactly 9 months", birth.AddDate(0, 9, 0), "9m"},
		{"exactly 12 months", birth.AddDate(1, 0, 0), "12m"},
		{"exactly 18 months", birth.AddDate(1, 6, 0), "18m"},
		{"just before 24 months", birth.AddDate(2, 0, -1), "18m"},
		{"exactly 24 months", birth.AddDate(2, 0, 0), "24m"},
		{"just before 3 years", birth.AddDate(3, 0, -1), "30m"},
		{"exactly 3 years", birth.AddDate(3, 0, 0), "3y"},
		{"exactly 11 years", birth.AddDate(11, 0, 0), "11y"},
		{"just before 12 years", birth.AddDate(12, 0, -1), "11y"},
		{"exactly 12 years", birth.AddDate(12, 0, 0), "12y+"},
		{"well past 12 years", birth.AddDate(30, 0, 0), "12y+"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := agerange.CodeFor(birth, tt.asOf)
			if got != tt.want {
				t.Errorf("CodeFor() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsValid(t *testing.T) {
	valid := []string{"0m", "18m", "3y", "12y+"}
	for _, code := range valid {
		if !agerange.IsValid(code) {
			t.Errorf("IsValid(%q) = false, want true", code)
		}
	}
	invalid := []string{"", "13y", "0y", "100m", "18M"}
	for _, code := range invalid {
		if agerange.IsValid(code) {
			t.Errorf("IsValid(%q) = true, want false", code)
		}
	}
}
