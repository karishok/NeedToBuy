// Package agerange computes which hardcoded age bucket a child's birth
// date currently falls into. The bucket grid is a fixed set of Go
// constants — no database table, no admin UI to edit it (product
// decision: see docs/mvp-decisions.md).
package agerange

import "time"

// boundary pairs a bucket's code (its lower bound) with how far after
// birth a child enters it, expressed as a calendar offset (years,
// months) rather than a day count, so month-length variation doesn't
// shift the boundary.
type boundary struct {
	code   string
	years  int
	months int
}

// boundaries is ordered from youngest to oldest. CodeFor walks it to
// find the last boundary a child has reached as of a given date — the
// implicit upper bound of any bucket is simply the next boundary's date.
var boundaries = []boundary{
	{"0m", 0, 0},
	{"1m", 0, 1},
	{"2m", 0, 2},
	{"3m", 0, 3},
	{"4m", 0, 4},
	{"5m", 0, 5},
	{"6m", 0, 6},
	{"9m", 0, 9},
	{"12m", 1, 0},
	{"15m", 1, 3},
	{"18m", 1, 6},
	{"24m", 2, 0},
	{"30m", 2, 6},
	{"3y", 3, 0},
	{"4y", 4, 0},
	{"5y", 5, 0},
	{"6y", 6, 0},
	{"7y", 7, 0},
	{"8y", 8, 0},
	{"9y", 9, 0},
	{"10y", 10, 0},
	{"11y", 11, 0},
	{"12y+", 12, 0},
}

// CodeFor returns the age-bucket code for a child born on birthDate, as
// of asOf — the lower bound of the bucket the child currently falls
// into. Assumes asOf is not before birthDate.
func CodeFor(birthDate, asOf time.Time) string {
	code := boundaries[0].code
	for _, b := range boundaries {
		boundaryDate := birthDate.AddDate(b.years, b.months, 0)
		if boundaryDate.After(asOf) {
			break
		}
		code = b.code
	}
	return code
}
