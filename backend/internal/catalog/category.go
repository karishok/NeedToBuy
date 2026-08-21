// Package catalog implements browsing the shared, curated catalog of
// gift ideas by age and category.
package catalog

// validCategories is the fixed set of catalog categories — a hardcoded
// Go enum, no database table, no admin UI to edit it (same product
// decision as internal/agerange's age grid — see docs/mvp-decisions.md).
var validCategories = map[string]bool{
	"clothes": true,
	"toys":    true,
	"books":   true,
	"sport":   true,
}

// IsValidCategory reports whether s is one of the four known categories.
func IsValidCategory(s string) bool {
	return validCategories[s]
}
