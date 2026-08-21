# Child Profile (backend) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Full CRUD for a parent's child profiles (name, birth date, computed age bucket, public share token, 152-ФЗ consent), on top of the existing Auth slice.

**Architecture:** A new `backend/internal/child` package owns the `children` table and its four HTTP endpoints, all mounted behind the existing `auth.Middleware` + `auth.RequireAuth`. A separate `backend/internal/agerange` package computes the age-bucket code live from `birth_date` — no stored, staling bucket. `auth.Querier` (already exported for exactly this purpose) gains one method (`SelectContext`) so `child`'s repository layer can reuse it instead of defining its own interface.

**Tech Stack:** Go, `chi`, `sqlx` over Postgres — matches the existing `backend/internal/auth` package's conventions exactly.

## Global Constraints

- Multiple children per parent — supported from the start, no artificial single-child limit.
- Full CRUD: `POST/GET /api/children`, `PATCH/DELETE /api/children/{id}`. No standalone `GET /api/children/{id}` — nothing consumes it yet.
- `consent` is required (`true`) on `POST`, otherwise `400 bad_request`. Not accepted on `PATCH` — consent is a one-time act at creation, never revoked via this API (only `DELETE`).
- `birth_date` must not be in the future; no lower bound (`12y+` covers everything older).
- `name`: trimmed, non-empty after trim, max 100 characters.
- Ownership: `GET` returns only the caller's children; `PATCH`/`DELETE` on a child owned by someone else (or that doesn't exist) return `404 not_found`, never `403` — don't confirm existence to a non-owner.
- `age_range_code` is computed fresh on every response from `birth_date` — never stored, never stale after a `PATCH`.
- Age bucket codes (lower bound + unit suffix, no upper bound in the code):
  `0m 1m 2m 3m 4m 5m 6m 9m 12m 15m 18m 24m 30m 3y 4y 5y 6y 7y 8y 9y 10y 11y 12y+`
- Every new/changed Go file keeps the existing repo convention: doc comment on the package and every exported identifier, errors wrapped with `fmt.Errorf("<pkg>: <verb>: %w", err)`.
- Source spec: [[docs/superpowers/specs/2026-08-21-child-profile-design.md]] (addendum to [[docs/superpowers/specs/2026-08-20-system-architecture-design.md]]).

---

## Task 1: Migration + extend `auth.Querier`

**Files:**
- Create: `backend/migrations/000004_create_children.up.sql`
- Create: `backend/migrations/000004_create_children.down.sql`
- Modify: `backend/internal/db/migrate_test.go`
- Modify: `backend/internal/auth/db.go`

**Interfaces:**
- Produces: the `children` table; `auth.Querier` gaining `SelectContext(ctx context.Context, dest any, query string, args ...any) error` alongside its existing `GetContext`/`ExecContext`. Task 3's repository layer (multi-row `GET /api/children`) needs this third method — `*sqlx.DB` and `*sqlx.Tx` both already implement it natively, so this is a pure widening, not a behavior change.

- [ ] **Step 1: Write the failing migration test**

Append to `backend/internal/db/migrate_test.go`:

```go
func TestMigrate_CreatesChildrenTable(t *testing.T) {
	dsn := dbtest.DSN(t)

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer conn.Close()

	var tableName string
	if err := conn.QueryRow("SELECT to_regclass('public.children')::text").Scan(&tableName); err != nil {
		t.Fatalf("query error = %v", err)
	}
	if tableName != "children" {
		t.Fatalf("expected children table to exist, got %q", tableName)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/db/... -run CreatesChildrenTable -v`
Expected: FAIL — `expected children table to exist, got ""`

- [ ] **Step 3: Write the migration**

`backend/migrations/000004_create_children.up.sql`:

```sql
CREATE TABLE children (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    parent_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    birth_date DATE NOT NULL,
    public_share_token TEXT NOT NULL UNIQUE,
    consent_child_data_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX children_parent_id_idx ON children (parent_id);
```

`backend/migrations/000004_create_children.down.sql`:

```sql
DROP TABLE children;
```

- [ ] **Step 4: Run the migration test to verify it passes**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/db/... -v`
Expected: PASS — all tests in `internal/db`, including the new one and the pre-existing `TestMigrate_IsIdempotent`.

- [ ] **Step 5: Extend `auth.Querier`**

In `backend/internal/auth/db.go`, add `SelectContext` to the `Querier` interface:

```go
// Querier is satisfied by both *sqlx.DB and *sqlx.Tx, so repository
// functions in this package run unchanged against a live connection in
// production and inside a rollback transaction in tests. It is exported
// so wrapper/decorator packages (logging, metrics) can reference it, e.g.
// to implement it themselves or assert `var _ auth.Querier = someType{}`.
type Querier interface {
	GetContext(ctx context.Context, dest any, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
```

- [ ] **Step 6: Run the full backend suite to confirm nothing broke**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./... -v`
Expected: PASS — every package (the `auth` package's existing call sites, which all pass real `*sqlx.DB`/`*sqlx.Tx`, keep compiling unchanged since both types already implement `SelectContext` natively).

- [ ] **Step 7: Commit**

```bash
git add migrations internal/db/migrate_test.go internal/auth/db.go
git commit -m "Add children migration; extend auth.Querier with SelectContext"
```

---

## Task 2: `internal/agerange` — age bucket computation

**Files:**
- Create: `backend/internal/agerange/agerange.go`
- Create: `backend/internal/agerange/agerange_test.go`

**Interfaces:**
- Produces: `agerange.CodeFor(birthDate, asOf time.Time) string`. Task 3's response-shaping code calls this directly.

- [ ] **Step 1: Write the failing tests**

`backend/internal/agerange/agerange_test.go`:

```go
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
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && go test ./internal/agerange/... -v`
Expected: FAIL — package doesn't exist / `undefined: agerange.CodeFor`

- [ ] **Step 3: Implement `agerange`**

`backend/internal/agerange/agerange.go`:

```go
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
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && go test ./internal/agerange/... -v`
Expected: PASS — all 16 table cases.

- [ ] **Step 5: Commit**

```bash
git add internal/agerange
git commit -m "Add agerange package: compute age bucket from birth date"
```

---

## Task 3: `internal/child` — repository layer

**Files:**
- Create: `backend/internal/child/child.go`
- Create: `backend/internal/child/validate.go`
- Create: `backend/internal/child/response.go`
- Create: `backend/internal/child/child_test.go`

**Interfaces:**
- Consumes: `auth.Querier` (Task 1), `agerange.CodeFor` (Task 2).
- Produces: `row` (unexported DB-row type), `errNotFound`, `createChild`, `listChildren`, `getChild`, `updateChild`, `deleteChild`, `generateShareToken`, `validateName`, `parseBirthDate`, `childResponse`, `toResponse` — all consumed by Task 4's HTTP handlers.

- [ ] **Step 1: Write the failing tests**

`backend/internal/child/child_test.go`:

```go
package child

import (
	"context"
	"errors"
	"testing"
	"time"

	"needtobuy/internal/auth"
	"needtobuy/internal/dbtest"
)

func mustCreateParent(t *testing.T, ctx context.Context, db auth.Querier, email string) int64 {
	t.Helper()
	var id int64
	if err := db.GetContext(ctx, &id, `INSERT INTO users (email) VALUES ($1) RETURNING id`, email); err != nil {
		t.Fatalf("create parent: %v", err)
	}
	return id
}

func TestCreateChild_ThenListChildren_ReturnsIt(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentID := mustCreateParent(t, ctx, tx, "parent@example.com")

	created, err := createChild(ctx, tx, parentID, "Тимофей", time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("createChild() error = %v", err)
	}
	if created.Name != "Тимофей" {
		t.Fatalf("Name = %q, want Тимофей", created.Name)
	}
	if created.PublicShareToken == "" {
		t.Fatal("PublicShareToken is empty, want a generated token")
	}

	rows, err := listChildren(ctx, tx, parentID)
	if err != nil {
		t.Fatalf("listChildren() error = %v", err)
	}
	if len(rows) != 1 || rows[0].ID != created.ID {
		t.Fatalf("listChildren() = %+v, want a single row matching id %d", rows, created.ID)
	}
}

func TestListChildren_IsolatesParents(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentA := mustCreateParent(t, ctx, tx, "a@example.com")
	parentB := mustCreateParent(t, ctx, tx, "b@example.com")

	if _, err := createChild(ctx, tx, parentA, "Child A", time.Now().AddDate(-2, 0, 0)); err != nil {
		t.Fatalf("createChild() error = %v", err)
	}

	rows, err := listChildren(ctx, tx, parentB)
	if err != nil {
		t.Fatalf("listChildren() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("listChildren() for parentB = %+v, want empty", rows)
	}
}

func TestUpdateChild_ChangesNameAndBirthDate(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentID := mustCreateParent(t, ctx, tx, "parent@example.com")
	created, err := createChild(ctx, tx, parentID, "Тимофей", time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("createChild() error = %v", err)
	}

	newName := "Тимур"
	newBirthDate := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
	updated, err := updateChild(ctx, tx, parentID, created.ID, &newName, &newBirthDate)
	if err != nil {
		t.Fatalf("updateChild() error = %v", err)
	}
	if updated.Name != "Тимур" {
		t.Fatalf("Name = %q, want Тимур", updated.Name)
	}
	if !updated.BirthDate.Equal(newBirthDate) {
		t.Fatalf("BirthDate = %v, want %v", updated.BirthDate, newBirthDate)
	}
}

func TestUpdateChild_WrongParent_ReturnsErrNotFound(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentA := mustCreateParent(t, ctx, tx, "a@example.com")
	parentB := mustCreateParent(t, ctx, tx, "b@example.com")
	created, err := createChild(ctx, tx, parentA, "Child A", time.Now().AddDate(-2, 0, 0))
	if err != nil {
		t.Fatalf("createChild() error = %v", err)
	}

	newName := "Hijacked"
	_, err = updateChild(ctx, tx, parentB, created.ID, &newName, nil)
	if !errors.Is(err, errNotFound) {
		t.Fatalf("updateChild() error = %v, want errNotFound", err)
	}
}

func TestDeleteChild_ThenListChildren_IsEmpty(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentID := mustCreateParent(t, ctx, tx, "parent@example.com")
	created, err := createChild(ctx, tx, parentID, "Тимофей", time.Now().AddDate(-2, 0, 0))
	if err != nil {
		t.Fatalf("createChild() error = %v", err)
	}

	if err := deleteChild(ctx, tx, parentID, created.ID); err != nil {
		t.Fatalf("deleteChild() error = %v", err)
	}

	rows, err := listChildren(ctx, tx, parentID)
	if err != nil {
		t.Fatalf("listChildren() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("listChildren() after delete = %+v, want empty", rows)
	}
}

func TestDeleteChild_WrongParent_ReturnsErrNotFound(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()
	parentA := mustCreateParent(t, ctx, tx, "a@example.com")
	parentB := mustCreateParent(t, ctx, tx, "b@example.com")
	created, err := createChild(ctx, tx, parentA, "Child A", time.Now().AddDate(-2, 0, 0))
	if err != nil {
		t.Fatalf("createChild() error = %v", err)
	}

	err = deleteChild(ctx, tx, parentB, created.ID)
	if !errors.Is(err, errNotFound) {
		t.Fatalf("deleteChild() error = %v, want errNotFound", err)
	}
}

func TestValidateName(t *testing.T) {
	if _, err := validateName("  "); err == nil {
		t.Error("validateName(\"  \") error = nil, want error for blank name")
	}
	if _, err := validateName(""); err == nil {
		t.Error("validateName(\"\") error = nil, want error for empty name")
	}
	got, err := validateName("  Тимофей  ")
	if err != nil {
		t.Fatalf("validateName() error = %v", err)
	}
	if got != "Тимофей" {
		t.Fatalf("validateName() = %q, want trimmed \"Тимофей\"", got)
	}
	tooLong := make([]byte, 101)
	for i := range tooLong {
		tooLong[i] = 'a'
	}
	if _, err := validateName(string(tooLong)); err == nil {
		t.Error("validateName() with 101 chars error = nil, want error")
	}
}

func TestParseBirthDate(t *testing.T) {
	if _, err := parseBirthDate("not-a-date"); err == nil {
		t.Error("parseBirthDate(\"not-a-date\") error = nil, want error")
	}
	future := time.Now().AddDate(1, 0, 0).Format("2006-01-02")
	if _, err := parseBirthDate(future); err == nil {
		t.Error("parseBirthDate() with future date error = nil, want error")
	}
	got, err := parseBirthDate("2024-03-15")
	if err != nil {
		t.Fatalf("parseBirthDate() error = %v", err)
	}
	want := time.Date(2024, time.March, 15, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("parseBirthDate() = %v, want %v", got, want)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/child/... -v`
Expected: FAIL — `undefined: createChild` (and the rest of the package's symbols)

- [ ] **Step 3: Implement `validate.go`, `response.go`, and `child.go`**

`backend/internal/child/validate.go`:

```go
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
```

`backend/internal/child/response.go`:

```go
package child

import (
	"time"

	"needtobuy/internal/agerange"
)

// childResponse is the JSON shape returned by every child-profile
// endpoint. age_range_code is computed fresh from birth_date on every
// response, never stored.
type childResponse struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	BirthDate        string    `json:"birth_date"`
	AgeRangeCode     string    `json:"age_range_code"`
	PublicShareToken string    `json:"public_share_token"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func toResponse(r row) childResponse {
	return childResponse{
		ID:               r.ID,
		Name:             r.Name,
		BirthDate:        r.BirthDate.Format("2006-01-02"),
		AgeRangeCode:     agerange.CodeFor(r.BirthDate, time.Now()),
		PublicShareToken: r.PublicShareToken,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}
```

`backend/internal/child/child.go`:

```go
// Package child implements CRUD for a parent's child profiles: name,
// birth date, the public share token a future wishlist share link will
// use, and consent to processing the child's personal data (152-ФЗ).
package child

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"needtobuy/internal/auth"
)

// errNotFound signals no matching child — missing, or owned by someone
// else. Handlers map it to a 404 without distinguishing the two cases,
// so a non-owner can't tell which one it is.
var errNotFound = errors.New("child: not found")

// row mirrors one row of the children table as scanned from Postgres.
type row struct {
	ID               int64     `db:"id"`
	ParentID         int64     `db:"parent_id"`
	Name             string    `db:"name"`
	BirthDate        time.Time `db:"birth_date"`
	PublicShareToken string    `db:"public_share_token"`
	CreatedAt        time.Time `db:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"`
}

const childColumns = "id, parent_id, name, birth_date, public_share_token, created_at, updated_at"

// generateShareToken returns a random opaque token for a child's public
// wishlist link, used directly as public_share_token.
func generateShareToken() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("child: generate share token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// createChild inserts a new child profile for parentID and returns it.
// Consent is assumed already validated true by the caller; this function
// always stamps consent_child_data_at to now().
func createChild(ctx context.Context, db auth.Querier, parentID int64, name string, birthDate time.Time) (row, error) {
	token, err := generateShareToken()
	if err != nil {
		return row{}, err
	}
	var r row
	err = db.GetContext(ctx, &r, `
		INSERT INTO children (parent_id, name, birth_date, public_share_token, consent_child_data_at)
		VALUES ($1, $2, $3, $4, now())
		RETURNING `+childColumns,
		parentID, name, birthDate, token)
	if err != nil {
		return row{}, fmt.Errorf("child: create: %w", err)
	}
	return r, nil
}

// listChildren returns all children belonging to parentID, oldest first.
func listChildren(ctx context.Context, db auth.Querier, parentID int64) ([]row, error) {
	var rows []row
	if err := db.SelectContext(ctx, &rows, `
		SELECT `+childColumns+`
		FROM children WHERE parent_id = $1 ORDER BY created_at`, parentID); err != nil {
		return nil, fmt.Errorf("child: list: %w", err)
	}
	return rows, nil
}

// getChild returns a single child owned by parentID. Returns errNotFound
// if it doesn't exist or belongs to someone else.
func getChild(ctx context.Context, db auth.Querier, parentID, id int64) (row, error) {
	var r row
	err := db.GetContext(ctx, &r, `
		SELECT `+childColumns+`
		FROM children WHERE id = $1 AND parent_id = $2`, id, parentID)
	if errors.Is(err, sql.ErrNoRows) {
		return row{}, errNotFound
	}
	if err != nil {
		return row{}, fmt.Errorf("child: get: %w", err)
	}
	return r, nil
}

// updateChild applies a partial update (name and/or birth date, whichever
// is non-nil) to a child owned by parentID, and returns the updated row.
// Returns errNotFound if id doesn't belong to parentID (or doesn't exist).
func updateChild(ctx context.Context, db auth.Querier, parentID, id int64, name *string, birthDate *time.Time) (row, error) {
	setClauses := []string{"updated_at = now()"}
	args := []any{}
	argN := 1
	if name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argN))
		args = append(args, *name)
		argN++
	}
	if birthDate != nil {
		setClauses = append(setClauses, fmt.Sprintf("birth_date = $%d", argN))
		args = append(args, *birthDate)
		argN++
	}
	args = append(args, id, parentID)
	query := fmt.Sprintf(`UPDATE children SET %s WHERE id = $%d AND parent_id = $%d`,
		strings.Join(setClauses, ", "), argN, argN+1)

	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return row{}, fmt.Errorf("child: update: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return row{}, fmt.Errorf("child: update rows affected: %w", err)
	}
	if n == 0 {
		return row{}, errNotFound
	}
	return getChild(ctx, db, parentID, id)
}

// deleteChild removes a child owned by parentID. Returns errNotFound if
// it doesn't exist or belongs to someone else.
func deleteChild(ctx context.Context, db auth.Querier, parentID, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM children WHERE id = $1 AND parent_id = $2`, id, parentID)
	if err != nil {
		return fmt.Errorf("child: delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("child: delete rows affected: %w", err)
	}
	if n == 0 {
		return errNotFound
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/child/... -v`
Expected: PASS — all tests.

- [ ] **Step 5: Commit**

```bash
git add internal/child
git commit -m "Add child profile repository layer"
```

---

## Task 4: `internal/child` — HTTP handlers

**Files:**
- Create: `backend/internal/child/handler.go`
- Create: `backend/internal/child/handler_test.go`

**Interfaces:**
- Consumes: `createChild`, `listChildren`, `updateChild`, `deleteChild`, `errNotFound`, `validateName`, `parseBirthDate`, `childResponse`, `toResponse` (Task 3); `auth.Querier`, `auth.UserID`, `auth.Handler`, `auth.Mailer` (existing `auth` package).
- Produces: `child.Handler{}`, `child.NewHandler(database auth.Querier) *Handler`, methods `(*Handler).Create`, `(*Handler).List`, `(*Handler).Update`, `(*Handler).Delete` — Task 5 registers these on the router.

- [ ] **Step 1: Write the failing tests**

`backend/internal/child/handler_test.go`:

```go
package child

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"needtobuy/internal/auth"
	"needtobuy/internal/dbtest"
)

// capturingMailer records the last code it was asked to send, so tests
// can read back the OTP that would have gone out by email.
type capturingMailer struct {
	lastCode string
}

func (m *capturingMailer) SendOTP(_ context.Context, _ string, code string) error {
	m.lastCode = code
	return nil
}

// testEnv bundles a child.Handler under test with a real auth.Handler
// sharing the same transaction, so tests can mint real session cookies
// and route requests through auth.Handler.Middleware exactly as
// production's router does — child handlers read the authenticated
// parent via auth.UserID(r.Context()), which only Middleware populates.
type testEnv struct {
	authHandler  *auth.Handler
	childHandler *Handler
	mailer       *capturingMailer
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tx := dbtest.Tx(t)
	mailer := &capturingMailer{}
	return &testEnv{
		authHandler:  auth.NewHandler(tx, mailer, "pepper"),
		childHandler: NewHandler(tx),
		mailer:       mailer,
	}
}

// login creates (or logs into) a parent by email and returns a session
// cookie for them.
func (e *testEnv) login(t *testing.T, email string) *http.Cookie {
	t.Helper()
	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"`+email+`"}`))
	e.authHandler.RequestOTP(httptest.NewRecorder(), reqReq)

	verifyBody := `{"email":"` + email + `","code":"` + e.mailer.lastCode + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	e.authHandler.VerifyOTP(verifyRec, verifyReq)
	return verifyRec.Result().Cookies()[0]
}

// serve wraps next in the real session middleware and drives req through
// it, so next can read auth.UserID(r.Context()).
func (e *testEnv) serve(next http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.authHandler.Middleware(next).ServeHTTP(rec, req)
	return rec
}

// withURLParam attaches a chi URL param the way the real router would,
// for handlers under test that read chi.URLParam(r, "id") without going
// through an actual chi router.
func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestCreate_ValidBody_Returns201(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t, "parent@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2024-03-15","consent":true}`))
	req.AddCookie(cookie)

	rec := env.serve(env.childHandler.Create, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["name"] != "Тимофей" {
		t.Fatalf("name = %v, want Тимофей", body["name"])
	}
	if body["public_share_token"] == "" || body["public_share_token"] == nil {
		t.Fatal("public_share_token is empty, want a generated token")
	}
}

func TestCreate_NoConsent_BadRequest(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t, "parent@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2024-03-15","consent":false}`))
	req.AddCookie(cookie)

	rec := env.serve(env.childHandler.Create, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreate_FutureBirthDate_BadRequest(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t, "parent@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2099-01-01","consent":true}`))
	req.AddCookie(cookie)

	rec := env.serve(env.childHandler.Create, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestList_ReturnsOnlyOwnChildren(t *testing.T) {
	env := newTestEnv(t)
	cookieA := env.login(t, "a@example.com")
	cookieB := env.login(t, "b@example.com")

	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Child A","birth_date":"2024-03-15","consent":true}`))
	createReq.AddCookie(cookieA)
	env.serve(env.childHandler.Create, createReq)

	listReq := httptest.NewRequest(http.MethodGet, "/api/children", nil)
	listReq.AddCookie(cookieB)
	rec := env.serve(env.childHandler.List, listReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var children []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &children); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(children) != 0 {
		t.Fatalf("children for B = %+v, want empty (A's child must not leak)", children)
	}
}

func TestUpdate_OwnChild_Returns200(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t, "parent@example.com")

	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2024-03-15","consent":true}`))
	createReq.AddCookie(cookie)
	createRec := env.serve(env.childHandler.Create, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	id := int64(created["id"].(float64))

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/children/"+strconv.FormatInt(id, 10),
		strings.NewReader(`{"name":"Тимур"}`))
	patchReq.AddCookie(cookie)
	patchReq = withURLParam(patchReq, "id", strconv.FormatInt(id, 10))
	rec := env.serve(env.childHandler.Update, patchReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var updated map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated["name"] != "Тимур" {
		t.Fatalf("name = %v, want Тимур", updated["name"])
	}
}

func TestUpdate_OtherParentsChild_NotFound(t *testing.T) {
	env := newTestEnv(t)
	cookieA := env.login(t, "a@example.com")
	cookieB := env.login(t, "b@example.com")

	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Child A","birth_date":"2024-03-15","consent":true}`))
	createReq.AddCookie(cookieA)
	createRec := env.serve(env.childHandler.Create, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	id := int64(created["id"].(float64))

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/children/"+strconv.FormatInt(id, 10),
		strings.NewReader(`{"name":"Hijacked"}`))
	patchReq.AddCookie(cookieB)
	patchReq = withURLParam(patchReq, "id", strconv.FormatInt(id, 10))
	rec := env.serve(env.childHandler.Update, patchReq)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestDelete_OwnChild_Returns200(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t, "parent@example.com")

	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2024-03-15","consent":true}`))
	createReq.AddCookie(cookie)
	createRec := env.serve(env.childHandler.Create, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	id := int64(created["id"].(float64))

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/children/"+strconv.FormatInt(id, 10), nil)
	deleteReq.AddCookie(cookie)
	deleteReq = withURLParam(deleteReq, "id", strconv.FormatInt(id, 10))
	rec := env.serve(env.childHandler.Delete, deleteReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestDelete_OtherParentsChild_NotFound(t *testing.T) {
	env := newTestEnv(t)
	cookieA := env.login(t, "a@example.com")
	cookieB := env.login(t, "b@example.com")

	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Child A","birth_date":"2024-03-15","consent":true}`))
	createReq.AddCookie(cookieA)
	createRec := env.serve(env.childHandler.Create, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	id := int64(created["id"].(float64))

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/children/"+strconv.FormatInt(id, 10), nil)
	deleteReq.AddCookie(cookieB)
	deleteReq = withURLParam(deleteReq, "id", strconv.FormatInt(id, 10))
	rec := env.serve(env.childHandler.Delete, deleteReq)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/child/... -v`
Expected: FAIL — `undefined: NewHandler` (and `Handler`)

- [ ] **Step 3: Implement `handler.go`**

```go
package child

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"needtobuy/internal/apierr"
	"needtobuy/internal/auth"
)

// Handler wires the child-profile HTTP endpoints to a database.
type Handler struct {
	db auth.Querier
}

// NewHandler builds a Handler ready to register on a router.
func NewHandler(database auth.Querier) *Handler {
	return &Handler{db: database}
}

type createBody struct {
	Name      string `json:"name"`
	BirthDate string `json:"birth_date"`
	Consent   bool   `json:"consent"`
}

// Create handles POST /api/children.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	parentID, _ := auth.UserID(r.Context())

	var body createBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierr.WriteError(w, apierr.BadRequest("invalid request body"))
		return
	}
	name, err := validateName(body.Name)
	if err != nil {
		apierr.WriteError(w, apierr.BadRequest(err.Error()))
		return
	}
	birthDate, err := parseBirthDate(body.BirthDate)
	if err != nil {
		apierr.WriteError(w, apierr.BadRequest(err.Error()))
		return
	}
	if !body.Consent {
		apierr.WriteError(w, apierr.BadRequest("consent is required"))
		return
	}

	created, err := createChild(r.Context(), h.db, parentID, name, birthDate)
	if err != nil {
		log.Printf("child: create for parent %d: %v", parentID, err)
		apierr.WriteError(w, apierr.Internal("could not create child"))
		return
	}
	apierr.WriteJSON(w, http.StatusCreated, toResponse(created))
}

// List handles GET /api/children.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	parentID, _ := auth.UserID(r.Context())

	rows, err := listChildren(r.Context(), h.db, parentID)
	if err != nil {
		log.Printf("child: list for parent %d: %v", parentID, err)
		apierr.WriteError(w, apierr.Internal("could not load children"))
		return
	}
	responses := make([]childResponse, len(rows))
	for i, row := range rows {
		responses[i] = toResponse(row)
	}
	apierr.WriteJSON(w, http.StatusOK, responses)
}

type updateBody struct {
	Name      *string `json:"name"`
	BirthDate *string `json:"birth_date"`
}

// Update handles PATCH /api/children/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	parentID, _ := auth.UserID(r.Context())
	id, err := parseID(r)
	if err != nil {
		apierr.WriteError(w, apierr.BadRequest("invalid child id"))
		return
	}

	var body updateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierr.WriteError(w, apierr.BadRequest("invalid request body"))
		return
	}

	var namePtr *string
	if body.Name != nil {
		name, err := validateName(*body.Name)
		if err != nil {
			apierr.WriteError(w, apierr.BadRequest(err.Error()))
			return
		}
		namePtr = &name
	}
	var birthDatePtr *time.Time
	if body.BirthDate != nil {
		bd, err := parseBirthDate(*body.BirthDate)
		if err != nil {
			apierr.WriteError(w, apierr.BadRequest(err.Error()))
			return
		}
		birthDatePtr = &bd
	}

	updated, err := updateChild(r.Context(), h.db, parentID, id, namePtr, birthDatePtr)
	if errors.Is(err, errNotFound) {
		apierr.WriteError(w, apierr.NotFound("child"))
		return
	}
	if err != nil {
		log.Printf("child: update %d for parent %d: %v", id, parentID, err)
		apierr.WriteError(w, apierr.Internal("could not update child"))
		return
	}
	apierr.WriteJSON(w, http.StatusOK, toResponse(updated))
}

// Delete handles DELETE /api/children/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	parentID, _ := auth.UserID(r.Context())
	id, err := parseID(r)
	if err != nil {
		apierr.WriteError(w, apierr.BadRequest("invalid child id"))
		return
	}

	err = deleteChild(r.Context(), h.db, parentID, id)
	if errors.Is(err, errNotFound) {
		apierr.WriteError(w, apierr.NotFound("child"))
		return
	}
	if err != nil {
		log.Printf("child: delete %d for parent %d: %v", id, parentID, err)
		apierr.WriteError(w, apierr.Internal("could not delete child"))
		return
	}
	apierr.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/child/... -v`
Expected: PASS — every test in `internal/child`.

- [ ] **Step 5: Commit**

```bash
git add internal/child/handler.go internal/child/handler_test.go
git commit -m "Add child profile HTTP handlers"
```

---

## Task 5: Wire into router and server entrypoint

**Files:**
- Modify: `backend/internal/httpapi/router.go`
- Modify: `backend/cmd/server/main.go`
- Test: `backend/internal/httpapi/child_flow_test.go`

**Interfaces:**
- Consumes: `child.NewHandler`, `(*child.Handler).Create/.List/.Update/.Delete` (Task 4).
- Produces: `httpapi.NewRouter(database *sqlx.DB, authHandler *auth.Handler, childHandler *child.Handler) http.Handler` — the signature future domain packages (wishlist, catalog) build on.

- [ ] **Step 1: Write the failing end-to-end test**

`backend/internal/httpapi/child_flow_test.go`:

```go
package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"needtobuy/internal/auth"
	"needtobuy/internal/child"
	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
	"needtobuy/internal/httpapi"
)

func TestChildFlow_CreateListUpdateDelete_EndToEnd(t *testing.T) {
	dsn := dbtest.DSN(t)
	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	mailer := &capturingMailer{}
	authHandler := auth.NewHandler(conn, mailer, "pepper")
	childHandler := child.NewHandler(conn)
	router := httpapi.NewRouter(conn, authHandler, childHandler)
	email := fmt.Sprintf("child-e2e-%d@example.com", time.Now().UnixNano())

	// Log in.
	reqBody := fmt.Sprintf(`{"email":%q}`, email)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("otp request: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	verifyBody := fmt.Sprintf(`{"email":%q,"code":%q}`, email, mailer.lastCode)
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	router.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("otp verify: status = %d, body = %s", verifyRec.Code, verifyRec.Body.String())
	}
	sessionCookie := verifyRec.Result().Cookies()[0]

	// No cookie at all: creating a child must be rejected.
	noCookieReq := httptest.NewRequest(http.MethodPost, "/api/children", strings.NewReader(`{"name":"x","birth_date":"2024-01-01","consent":true}`))
	noCookieRec := httptest.NewRecorder()
	router.ServeHTTP(noCookieRec, noCookieReq)
	if noCookieRec.Code != http.StatusUnauthorized {
		t.Fatalf("create without cookie: status = %d, want %d", noCookieRec.Code, http.StatusUnauthorized)
	}

	// Create a child.
	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2024-03-15","consent":true}`))
	createReq.AddCookie(sessionCookie)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	id := int64(created["id"].(float64))
	if created["age_range_code"] == "" {
		t.Fatal("age_range_code is empty")
	}

	// List: the new child appears.
	listReq := httptest.NewRequest(http.MethodGet, "/api/children", nil)
	listReq.AddCookie(sessionCookie)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list: status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	var listed []map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list = %+v, want exactly 1 child", listed)
	}

	// Update the child's name.
	patchReq := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/children/%d", id),
		strings.NewReader(`{"name":"Тимур"}`))
	patchReq.AddCookie(sessionCookie)
	patchRec := httptest.NewRecorder()
	router.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch: status = %d, body = %s", patchRec.Code, patchRec.Body.String())
	}

	// Delete the child.
	deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/children/%d", id), nil)
	deleteReq.AddCookie(sessionCookie)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete: status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}

	// List again: empty.
	listReq2 := httptest.NewRequest(http.MethodGet, "/api/children", nil)
	listReq2.AddCookie(sessionCookie)
	listRec2 := httptest.NewRecorder()
	router.ServeHTTP(listRec2, listReq2)
	var listed2 []map[string]any
	if err := json.Unmarshal(listRec2.Body.Bytes(), &listed2); err != nil {
		t.Fatalf("unmarshal second list response: %v", err)
	}
	if len(listed2) != 0 {
		t.Fatalf("list after delete = %+v, want empty", listed2)
	}
}
```

(`capturingMailer` is already defined in this package's `auth_flow_test.go` from the Auth slice — this new file reuses it, no redefinition needed.)

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/httpapi/... -v`
Expected: FAIL — `httpapi.NewRouter(conn, authHandler, childHandler)`: too many arguments (current signature only takes two)

- [ ] **Step 3: Update `router.go`**

Replace `backend/internal/httpapi/router.go` with:

```go
// Package httpapi implements the top-level HTTP router and health check.
package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"

	"needtobuy/internal/apierr"
	"needtobuy/internal/auth"
	"needtobuy/internal/child"
)

// NewRouter builds the top-level chi router. database is used by the
// health check to verify connectivity; authHandler registers the OTP,
// logout, and me endpoints and its Middleware runs on every request so
// downstream handlers can read the authenticated parent via
// auth.UserID; childHandler registers the child-profile CRUD endpoints
// behind auth.RequireAuth.
func NewRouter(database *sqlx.DB, authHandler *auth.Handler, childHandler *child.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(authHandler.Middleware)

	r.Get("/healthz", healthHandler(database))

	r.Post("/api/auth/otp/request", authHandler.RequestOTP)
	r.Post("/api/auth/otp/verify", authHandler.VerifyOTP)
	r.Post("/api/auth/logout", authHandler.Logout)
	r.With(auth.RequireAuth).Get("/api/auth/me", authHandler.Me)

	r.Route("/api/children", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Post("/", childHandler.Create)
		r.Get("/", childHandler.List)
		r.Patch("/{id}", childHandler.Update)
		r.Delete("/{id}", childHandler.Delete)
	})

	return r
}

func healthHandler(database *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := database.PingContext(r.Context()); err != nil {
			apierr.WriteError(w, apierr.Internal("database unavailable"))
			return
		}
		apierr.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
```

- [ ] **Step 4: Update `router_test.go` for the new signature**

Replace `backend/internal/httpapi/router_test.go` with:

```go
package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"needtobuy/internal/auth"
	"needtobuy/internal/child"
	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
	"needtobuy/internal/httpapi"
)

type noopMailer struct{}

func (noopMailer) SendOTP(_ context.Context, _ string, _ string) error { return nil }

func TestHealthz_OK(t *testing.T) {
	dsn := dbtest.DSN(t)

	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	authHandler := auth.NewHandler(conn, noopMailer{}, "test-pepper")
	childHandler := child.NewHandler(conn)
	router := httpapi.NewRouter(conn, authHandler, childHandler)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestHealthz_DatabaseDown(t *testing.T) {
	dsn := dbtest.DSN(t)

	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	authHandler := auth.NewHandler(conn, noopMailer{}, "test-pepper")
	childHandler := child.NewHandler(conn)
	conn.Close() // force the ping in the handler to fail

	router := httpapi.NewRouter(conn, authHandler, childHandler)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}
```

- [ ] **Step 5: Update `main.go`**

Replace `backend/cmd/server/main.go` with:

```go
// Command server runs the NeedToBuy HTTP API.
package main

import (
	"log"
	"net/http"

	"needtobuy/internal/auth"
	"needtobuy/internal/child"
	"needtobuy/internal/config"
	"needtobuy/internal/db"
	"needtobuy/internal/httpapi"
)

func main() {
	cfg := config.Load()

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer database.Close()

	mailer := auth.SMTPMailer{Addr: cfg.SMTPAddr, From: cfg.SMTPFrom}
	authHandler := auth.NewHandler(database, mailer, cfg.OTPPepper)
	childHandler := child.NewHandler(database)

	router := httpapi.NewRouter(database, authHandler, childHandler)

	log.Printf("listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
```

- [ ] **Step 6: Run the full backend test suite**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./... -v`
Expected: PASS — every package (`apierr`, `agerange`, `auth`, `child`, `config`, `db`, `dbtest`, `httpapi`).

- [ ] **Step 7: Verify the server builds and runs**

Run (from `backend/`): `go build ./... && echo BUILD_OK`
Expected: `BUILD_OK`, no errors.

- [ ] **Step 8: Commit**

```bash
git add internal/httpapi cmd/server
git commit -m "Wire child profile CRUD into the router"
```

---

## Self-review notes

- **Spec coverage:** multi-child support (no unique constraint on `parent_id`) → Task 1's migration; full CRUD → Tasks 3-5; `consent` required on create, not on patch → Task 4's `Create`/`Update`; ownership → 404 semantics tested in Tasks 3 and 4; age bucket codes and computation → Task 2, consumed by Task 3's `toResponse`; `birth_date` not in the future → `parseBirthDate` in Task 3.
- **`auth.Querier` extension:** flagged explicitly in Task 1 as a widening, not a breaking change — existing call sites in `auth` (which pass real `*sqlx.DB`/`*sqlx.Tx`) keep compiling unchanged.
- **Cross-task type consistency:** `row`/`childResponse`/`errNotFound` (Task 3) are used identically by Task 4's handlers; `child.NewHandler`/`Handler` (Task 4) match Task 5's router wiring exactly; `agerange.CodeFor(birthDate, asOf time.Time) string` (Task 2) matches its one call site in Task 3's `response.go`.
- **Out of scope, confirmed unblocked for later slices:** frontend onboarding (needs `POST /api/children` — ready), wishlist (`wishlist_items.child_id` FK — this slice's `children.id` is ready to reference), public gifter view (needs `public_share_token` — generated and stored here, endpoint itself not built).
