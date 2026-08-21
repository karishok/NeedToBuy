package catalog

import "testing"

func TestIsValidCategory(t *testing.T) {
	valid := []string{"clothes", "toys", "books", "sport"}
	for _, c := range valid {
		if !IsValidCategory(c) {
			t.Errorf("IsValidCategory(%q) = false, want true", c)
		}
	}
	invalid := []string{"", "shoes", "Toys", "CLOTHES"}
	for _, c := range invalid {
		if IsValidCategory(c) {
			t.Errorf("IsValidCategory(%q) = true, want false", c)
		}
	}
}
