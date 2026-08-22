# Catalog Admin Panel Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the single admin (Karina) manually add, edit, and publish/hide catalog items — including upgrading the catalog's data model from "one item = one age + one category" to "one item can belong to several age ranges and categories at once."

**Architecture:** The `catalog_items` table drops its `age_range_code`/`category` columns in favor of two many-to-many join tables (`catalog_item_age_ranges`, `catalog_item_categories`) and gains a `status` column (`published`/`hidden`). The existing public `internal/catalog` package grows admin-only repository functions and handler methods (`ListAdmin`, `Create`, `Update`) alongside its existing public `List`. A new `auth.RequireAdmin` middleware gates `/api/admin/*` by comparing the session's email to a single configured admin email — no roles table. The frontend gets a new `/admin` route (visible only to the admin, guarded client-side too) with a list + inline create/edit form, and the existing public catalog card is updated to render multiple tags per item.

**Tech Stack:** Go, chi, sqlx, PostgreSQL (backend — matches every prior slice's conventions exactly). React, TypeScript, Vitest, React Testing Library (frontend — matches every prior slice's conventions exactly).

## Global Constraints

- `catalog_items` loses `age_range_code`/`category`; tags live in `catalog_item_age_ranges(catalog_item_id, age_range_code)` and `catalog_item_categories(catalog_item_id, category)` — composite-PK join tables, no FK to a codes table (validated in Go via the existing `agerange.IsValid`/`catalog.IsValidCategory`).
- `status` is `published`/`hidden` only, validated in Go — no DB CHECK constraint, no `source`/`approved_at` columns (deferred to a future AI-generation slice).
- Public `GET /api/catalog` returns only `status='published'` items and never exposes `status` in its JSON. Admin endpoints return every item plus `status`.
- `/api/admin/catalog*` requires a session (`auth.RequireAuth`) AND the session's email must equal `config.AdminEmail` (default `babaliants@gmail.com`, override via `ADMIN_EMAIL` env var) — anonymous → 401, authenticated non-admin → 403.
- No delete endpoint — only the `published`/`hidden` toggle via `PATCH`.
- Photo is a plain URL text field (`image_url`), no file upload, no storage service.
- `PATCH /api/admin/catalog/{id}`: a field present in the JSON body replaces its current value entirely (tags: full replace, not merge); a field absent from the JSON body is left untouched.
- `age_range_codes`/`categories` on create must be non-empty arrays of valid codes; same when included on update.
- No pagination on `GET /api/admin/catalog` — dataset stays small (manual, one item at a time).
- Source spec: [[docs/superpowers/specs/2026-08-22-catalog-admin-design.md]] (extends [[docs/superpowers/specs/2026-08-21-catalog-design.md]]).

---

## Task 1: Migration 000008 — multi-tag schema + status

**Files:**
- Create: `backend/migrations/000008_catalog_items_tags_and_status.up.sql`
- Create: `backend/migrations/000008_catalog_items_tags_and_status.down.sql`
- Modify: `backend/internal/db/migrate_test.go`

**Interfaces:**
- Produces: `catalog_items.status` column; `catalog_item_age_ranges`/`catalog_item_categories` tables, populated from the 18 existing seed rows' single age/category pair. Task 2's repository layer queries these directly.

- [ ] **Step 1: Write the failing test**

Append to `backend/internal/db/migrate_test.go`:

```go
func TestMigrate_CreatesCatalogItemTagTables(t *testing.T) {
	dsn := dbtest.DSN(t)

	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	conn, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	defer conn.Close()

	for _, table := range []string{"catalog_item_age_ranges", "catalog_item_categories"} {
		var tableName string
		if err := conn.QueryRow("SELECT to_regclass('public." + table + "')::text").Scan(&tableName); err != nil {
			t.Fatalf("query error for %s: %v", table, err)
		}
		if tableName != table {
			t.Fatalf("expected %s table to exist, got %q", table, tableName)
		}
	}

	var ageRangeCount int
	if err := conn.QueryRow("SELECT count(*) FROM catalog_item_age_ranges").Scan(&ageRangeCount); err != nil {
		t.Fatalf("count query error = %v", err)
	}
	if ageRangeCount == 0 {
		t.Fatal("expected seed items' age ranges migrated into catalog_item_age_ranges, got 0 rows")
	}

	var categoryCount int
	if err := conn.QueryRow("SELECT count(*) FROM catalog_item_categories").Scan(&categoryCount); err != nil {
		t.Fatalf("count query error = %v", err)
	}
	if categoryCount == 0 {
		t.Fatal("expected seed items' categories migrated into catalog_item_categories, got 0 rows")
	}

	var statusCount int
	if err := conn.QueryRow("SELECT count(*) FROM catalog_items WHERE status = 'published'").Scan(&statusCount); err != nil {
		t.Fatalf("count query error = %v", err)
	}
	if statusCount == 0 {
		t.Fatal("expected existing seed items to default to status = 'published'")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/db/... -run TestMigrate_CreatesCatalogItemTagTables -v`
Expected: FAIL — `catalog_item_age_ranges` doesn't exist yet.

- [ ] **Step 3: Write the migration**

`backend/migrations/000008_catalog_items_tags_and_status.up.sql`:

```sql
ALTER TABLE catalog_items ADD COLUMN status TEXT NOT NULL DEFAULT 'published';

CREATE TABLE catalog_item_age_ranges (
    catalog_item_id BIGINT NOT NULL REFERENCES catalog_items(id) ON DELETE CASCADE,
    age_range_code TEXT NOT NULL,
    PRIMARY KEY (catalog_item_id, age_range_code)
);

CREATE TABLE catalog_item_categories (
    catalog_item_id BIGINT NOT NULL REFERENCES catalog_items(id) ON DELETE CASCADE,
    category TEXT NOT NULL,
    PRIMARY KEY (catalog_item_id, category)
);

CREATE INDEX catalog_item_age_ranges_code_idx ON catalog_item_age_ranges (age_range_code);
CREATE INDEX catalog_item_categories_category_idx ON catalog_item_categories (category);

-- Carry each existing item's single age_range_code/category into the new
-- join tables before dropping the old columns.
INSERT INTO catalog_item_age_ranges (catalog_item_id, age_range_code)
SELECT id, age_range_code FROM catalog_items;

INSERT INTO catalog_item_categories (catalog_item_id, category)
SELECT id, category FROM catalog_items;

DROP INDEX IF EXISTS catalog_items_filter_idx;
ALTER TABLE catalog_items DROP COLUMN age_range_code;
ALTER TABLE catalog_items DROP COLUMN category;
```

`backend/migrations/000008_catalog_items_tags_and_status.down.sql`:

```sql
ALTER TABLE catalog_items ADD COLUMN age_range_code TEXT;
ALTER TABLE catalog_items ADD COLUMN category TEXT;

-- Lossy on purpose: an item with multiple tags keeps only its
-- alphabetically-first age range and category when reverting.
UPDATE catalog_items ci SET
    age_range_code = (
        SELECT age_range_code FROM catalog_item_age_ranges
        WHERE catalog_item_id = ci.id ORDER BY age_range_code LIMIT 1
    ),
    category = (
        SELECT category FROM catalog_item_categories
        WHERE catalog_item_id = ci.id ORDER BY category LIMIT 1
    );

ALTER TABLE catalog_items ALTER COLUMN age_range_code SET NOT NULL;
ALTER TABLE catalog_items ALTER COLUMN category SET NOT NULL;

CREATE INDEX catalog_items_filter_idx ON catalog_items (age_range_code, category);

DROP TABLE catalog_item_categories;
DROP TABLE catalog_item_age_ranges;

ALTER TABLE catalog_items DROP COLUMN status;
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/db/... -v`
Expected: PASS — all tests in `internal/db`, including the new one and the pre-existing `TestMigrate_CreatesCatalogItemsTable` (unaffected — it never referenced the dropped columns).

- [ ] **Step 5: Commit**

```bash
git add migrations internal/db/migrate_test.go
git commit -m "Add catalog_items status and multi-tag join tables"
```

---

## Task 2: Repository layer — multi-tag catalog data access

**Files:**
- Create: `backend/internal/catalog/status.go`
- Create: `backend/internal/catalog/status_test.go`
- Create: `backend/internal/catalog/validate.go`
- Create: `backend/internal/catalog/validate_test.go`
- Modify: `backend/internal/catalog/catalog.go` (full replacement)
- Modify: `backend/internal/catalog/catalog_test.go` (full replacement)

**Interfaces:**
- Consumes: `auth.Querier`, `agerange.IsValid` (existing); `IsValidCategory` (existing, in `category.go`, unchanged).
- Produces: `StatusPublished`, `StatusHidden` string constants, `IsValidStatus(s string) bool`; `validateTitle`, `validateURL`, `validateAgeRangeCodes`, `validateCategories` (unexported); `item` struct `{ID int64, Title string, MarketplaceSearchURL string, ImageURL sql.NullString, Status string, AgeRangeCodes []string, Categories []string}`; `listCatalogItems(ctx, db, ageRange, category string) ([]item, error)` (published-only, filtered); `listAllCatalogItems(ctx, db) ([]item, error)` (every status); `createCatalogItem(ctx, db, title, url, imageURL string, ageRanges, categories []string, status string) (item, error)`; `updateCatalogItem(ctx, db, id int64, title, url, imageURL, status *string, ageRanges, categories *[]string) (item, error)`; `errNotFound` sentinel error. Task 3 consumes all of these.

- [ ] **Step 1: Write the failing tests**

`backend/internal/catalog/status_test.go`:

```go
package catalog

import "testing"

func TestIsValidStatus(t *testing.T) {
	for _, s := range []string{"published", "hidden"} {
		if !IsValidStatus(s) {
			t.Errorf("IsValidStatus(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "draft", "PUBLISHED"} {
		if IsValidStatus(s) {
			t.Errorf("IsValidStatus(%q) = true, want false", s)
		}
	}
}
```

`backend/internal/catalog/validate_test.go`:

```go
package catalog

import "testing"

func TestValidateTitle(t *testing.T) {
	if _, err := validateTitle("  Сортер  "); err != nil {
		t.Fatalf("validateTitle() error = %v, want nil", err)
	}
	if got, _ := validateTitle("  Сортер  "); got != "Сортер" {
		t.Fatalf("validateTitle() = %q, want trimmed", got)
	}
	if _, err := validateTitle("   "); err == nil {
		t.Fatal("validateTitle(\"   \") error = nil, want an error")
	}
}

func TestValidateURL(t *testing.T) {
	if _, err := validateURL("https://ozon.ru/search"); err != nil {
		t.Fatalf("validateURL() error = %v, want nil", err)
	}
	if _, err := validateURL(""); err == nil {
		t.Fatal("validateURL(\"\") error = nil, want an error")
	}
}

func TestValidateAgeRangeCodes(t *testing.T) {
	if err := validateAgeRangeCodes([]string{"12m", "18m"}); err != nil {
		t.Fatalf("validateAgeRangeCodes() error = %v, want nil", err)
	}
	if err := validateAgeRangeCodes(nil); err == nil {
		t.Fatal("validateAgeRangeCodes(nil) error = nil, want an error (empty)")
	}
	if err := validateAgeRangeCodes([]string{"12m", "not-a-code"}); err == nil {
		t.Fatal("validateAgeRangeCodes with an unknown code error = nil, want an error")
	}
}

func TestValidateCategories(t *testing.T) {
	if err := validateCategories([]string{"toys", "books"}); err != nil {
		t.Fatalf("validateCategories() error = %v, want nil", err)
	}
	if err := validateCategories(nil); err == nil {
		t.Fatal("validateCategories(nil) error = nil, want an error (empty)")
	}
	if err := validateCategories([]string{"shoes"}); err == nil {
		t.Fatal("validateCategories with an unknown category error = nil, want an error")
	}
}
```

Replace `backend/internal/catalog/catalog_test.go` entirely with:

```go
package catalog

import (
	"context"
	"testing"

	"needtobuy/internal/dbtest"
)

func TestListCatalogItems_NoFilters_ReturnsOnlyPublished(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	hidden, err := createCatalogItem(ctx, tx, "Скрытый товар", "https://example.com", "", []string{"18m"}, []string{"toys"}, StatusHidden)
	if err != nil {
		t.Fatalf("createCatalogItem() error = %v", err)
	}

	items, err := listCatalogItems(ctx, tx, "", "")
	if err != nil {
		t.Fatalf("listCatalogItems() error = %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected the seeded published items, got none")
	}
	for _, it := range items {
		if it.ID == hidden.ID {
			t.Fatalf("listCatalogItems() included hidden item %d, want it excluded", hidden.ID)
		}
	}
}

func TestListCatalogItems_FiltersByAgeRange_MatchesAnyTag(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	created, err := createCatalogItem(ctx, tx, "Мультивозрастной товар", "https://example.com", "",
		[]string{"12m", "18m"}, []string{"books"}, StatusPublished)
	if err != nil {
		t.Fatalf("createCatalogItem() error = %v", err)
	}

	for _, code := range []string{"12m", "18m"} {
		items, err := listCatalogItems(ctx, tx, code, "")
		if err != nil {
			t.Fatalf("listCatalogItems(%q) error = %v", code, err)
		}
		found := false
		for _, it := range items {
			if it.ID == created.ID {
				found = true
			}
		}
		if !found {
			t.Fatalf("listCatalogItems(%q) did not include item %d tagged with both 12m and 18m", code, created.ID)
		}
	}
}

func TestListCatalogItems_FiltersByCategory_MatchesAnyTag(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	created, err := createCatalogItem(ctx, tx, "Мультикатегорийный товар", "https://example.com", "",
		[]string{"18m"}, []string{"books", "toys"}, StatusPublished)
	if err != nil {
		t.Fatalf("createCatalogItem() error = %v", err)
	}

	for _, cat := range []string{"books", "toys"} {
		items, err := listCatalogItems(ctx, tx, "", cat)
		if err != nil {
			t.Fatalf("listCatalogItems(%q) error = %v", cat, err)
		}
		found := false
		for _, it := range items {
			if it.ID == created.ID {
				found = true
			}
		}
		if !found {
			t.Fatalf("listCatalogItems(category=%q) did not include item %d tagged with both books and toys", cat, created.ID)
		}
	}
}

func TestListCatalogItems_FiltersByBoth_NoMatch_ReturnsEmpty(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	created, err := createCatalogItem(ctx, tx, "Одна пара", "https://example.com", "",
		[]string{"18m"}, []string{"toys"}, StatusPublished)
	if err != nil {
		t.Fatalf("createCatalogItem() error = %v", err)
	}

	items, err := listCatalogItems(ctx, tx, "18m", "books")
	if err != nil {
		t.Fatalf("listCatalogItems() error = %v", err)
	}
	for _, it := range items {
		if it.ID == created.ID {
			t.Fatalf("listCatalogItems(18m, books) unexpectedly matched item %d, which is only toys", created.ID)
		}
	}
}

func TestListAllCatalogItems_IncludesHidden(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	hidden, err := createCatalogItem(ctx, tx, "Скрытый для админки", "https://example.com", "", []string{"18m"}, []string{"toys"}, StatusHidden)
	if err != nil {
		t.Fatalf("createCatalogItem() error = %v", err)
	}

	items, err := listAllCatalogItems(ctx, tx)
	if err != nil {
		t.Fatalf("listAllCatalogItems() error = %v", err)
	}
	found := false
	for _, it := range items {
		if it.ID == hidden.ID {
			found = true
			if it.Status != StatusHidden {
				t.Fatalf("item %d Status = %q, want %q", it.ID, it.Status, StatusHidden)
			}
		}
	}
	if !found {
		t.Fatalf("listAllCatalogItems() did not include hidden item %d", hidden.ID)
	}
}

func TestCreateCatalogItem_ReturnsItemWithTags(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	created, err := createCatalogItem(ctx, tx, "Новый товар", "https://ozon.ru/search/?text=x", "https://example.com/photo.jpg",
		[]string{"12m", "18m"}, []string{"books", "toys"}, StatusPublished)
	if err != nil {
		t.Fatalf("createCatalogItem() error = %v", err)
	}
	if created.Title != "Новый товар" {
		t.Fatalf("Title = %q, want %q", created.Title, "Новый товар")
	}
	if !created.ImageURL.Valid || created.ImageURL.String != "https://example.com/photo.jpg" {
		t.Fatalf("ImageURL = %+v, want a valid photo URL", created.ImageURL)
	}
	if len(created.AgeRangeCodes) != 2 || len(created.Categories) != 2 {
		t.Fatalf("AgeRangeCodes/Categories = %v/%v, want 2 each", created.AgeRangeCodes, created.Categories)
	}
}

func TestCreateCatalogItem_EmptyImageURL_StoresNull(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	created, err := createCatalogItem(ctx, tx, "Без фото", "https://example.com", "", []string{"18m"}, []string{"toys"}, StatusPublished)
	if err != nil {
		t.Fatalf("createCatalogItem() error = %v", err)
	}
	if created.ImageURL.Valid {
		t.Fatalf("ImageURL = %+v, want NULL for an empty image_url", created.ImageURL)
	}
}

func TestUpdateCatalogItem_PartialUpdate_OnlyChangesGivenFields(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	created, err := createCatalogItem(ctx, tx, "Исходный заголовок", "https://example.com", "", []string{"18m"}, []string{"toys"}, StatusPublished)
	if err != nil {
		t.Fatalf("createCatalogItem() error = %v", err)
	}

	newTitle := "Новый заголовок"
	updated, err := updateCatalogItem(ctx, tx, created.ID, &newTitle, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("updateCatalogItem() error = %v", err)
	}
	if updated.Title != "Новый заголовок" {
		t.Fatalf("Title = %q, want %q", updated.Title, "Новый заголовок")
	}
	if updated.MarketplaceSearchURL != "https://example.com" {
		t.Fatalf("MarketplaceSearchURL = %q, want unchanged", updated.MarketplaceSearchURL)
	}
	if len(updated.AgeRangeCodes) != 1 || updated.AgeRangeCodes[0] != "18m" {
		t.Fatalf("AgeRangeCodes = %v, want unchanged [18m]", updated.AgeRangeCodes)
	}
}

func TestUpdateCatalogItem_ReplacesTagSet(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	created, err := createCatalogItem(ctx, tx, "Товар", "https://example.com", "", []string{"18m", "24m"}, []string{"toys"}, StatusPublished)
	if err != nil {
		t.Fatalf("createCatalogItem() error = %v", err)
	}

	newAgeRanges := []string{"3y"}
	updated, err := updateCatalogItem(ctx, tx, created.ID, nil, nil, nil, nil, &newAgeRanges, nil)
	if err != nil {
		t.Fatalf("updateCatalogItem() error = %v", err)
	}
	if len(updated.AgeRangeCodes) != 1 || updated.AgeRangeCodes[0] != "3y" {
		t.Fatalf("AgeRangeCodes = %v, want exactly [3y] (full replace, not merge)", updated.AgeRangeCodes)
	}
}

func TestUpdateCatalogItem_TogglesStatus(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	created, err := createCatalogItem(ctx, tx, "Товар", "https://example.com", "", []string{"18m"}, []string{"toys"}, StatusPublished)
	if err != nil {
		t.Fatalf("createCatalogItem() error = %v", err)
	}

	hidden := StatusHidden
	updated, err := updateCatalogItem(ctx, tx, created.ID, nil, nil, nil, &hidden, nil, nil)
	if err != nil {
		t.Fatalf("updateCatalogItem() error = %v", err)
	}
	if updated.Status != StatusHidden {
		t.Fatalf("Status = %q, want %q", updated.Status, StatusHidden)
	}
}

func TestUpdateCatalogItem_UnknownID_ReturnsErrNotFound(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	title := "x"
	_, err := updateCatalogItem(ctx, tx, 999999999, &title, nil, nil, nil, nil, nil)
	if err != errNotFound {
		t.Fatalf("updateCatalogItem() error = %v, want errNotFound", err)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/catalog/... -v`
Expected: FAIL — `undefined: IsValidStatus`, `undefined: validateTitle`, `undefined: createCatalogItem`, etc. (and the old `catalog_test.go`'s replaced content won't yet compile against the still-old `catalog.go`).

- [ ] **Step 3: Implement `status.go`, `validate.go`, and rewrite `catalog.go`**

`backend/internal/catalog/status.go`:

```go
package catalog

// The two states a catalog item can be in. No "draft" — this package has
// no AI-generation workflow (see docs/superpowers/specs/2026-08-22-catalog-admin-design.md),
// so every item is either live on the public catalog or not.
const (
	StatusPublished = "published"
	StatusHidden    = "hidden"
)

// IsValidStatus reports whether s is one of the two known statuses.
func IsValidStatus(s string) bool {
	return s == StatusPublished || s == StatusHidden
}
```

`backend/internal/catalog/validate.go`:

```go
package catalog

import (
	"errors"
	"fmt"
	"strings"

	"needtobuy/internal/agerange"
)

func validateTitle(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("title is required")
	}
	return s, nil
}

func validateURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("marketplace_search_url is required")
	}
	return s, nil
}

// validateAgeRangeCodes requires at least one code, each a known
// agerange bucket — an item with no age ranges could never appear in the
// public catalog, which is almost always an input mistake.
func validateAgeRangeCodes(codes []string) error {
	if len(codes) == 0 {
		return errors.New("age_range_codes must have at least one value")
	}
	for _, c := range codes {
		if !agerange.IsValid(c) {
			return fmt.Errorf("age_range_codes contains an unknown code: %q", c)
		}
	}
	return nil
}

// validateCategories requires at least one category, each one of the
// four known values — same rationale as validateAgeRangeCodes.
func validateCategories(categories []string) error {
	if len(categories) == 0 {
		return errors.New("categories must have at least one value")
	}
	for _, c := range categories {
		if !IsValidCategory(c) {
			return fmt.Errorf("categories contains an unknown category: %q", c)
		}
	}
	return nil
}
```

Replace `backend/internal/catalog/catalog.go` entirely with:

```go
package catalog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"needtobuy/internal/auth"
)

// row mirrors one row of the catalog_items table, before its tags are
// attached.
type row struct {
	ID                   int64          `db:"id"`
	Title                string         `db:"title"`
	MarketplaceSearchURL string         `db:"marketplace_search_url"`
	ImageURL             sql.NullString `db:"image_url"`
	Status               string         `db:"status"`
}

// item bundles a catalog_items row with its full set of age-range and
// category tags, loaded from the join tables.
type item struct {
	ID                   int64
	Title                string
	MarketplaceSearchURL string
	ImageURL             sql.NullString
	Status               string
	AgeRangeCodes        []string
	Categories           []string
}

// errNotFound is returned by updateCatalogItem when no catalog item has
// the given id.
var errNotFound = errors.New("catalog: item not found")

// listCatalogItems returns published catalog items matching the given
// filters. An empty ageRange or category means "no filter on that
// field." A non-empty filter matches an item that has that code among
// its (possibly several) tags.
func listCatalogItems(ctx context.Context, db auth.Querier, ageRange, category string) ([]item, error) {
	query := `SELECT DISTINCT ci.id, ci.title, ci.marketplace_search_url, ci.image_url, ci.status FROM catalog_items ci`
	args := []any{}
	argN := 1
	if ageRange != "" {
		query += fmt.Sprintf(" JOIN catalog_item_age_ranges ar ON ar.catalog_item_id = ci.id AND ar.age_range_code = $%d", argN)
		args = append(args, ageRange)
		argN++
	}
	if category != "" {
		query += fmt.Sprintf(" JOIN catalog_item_categories cat ON cat.catalog_item_id = ci.id AND cat.category = $%d", argN)
		args = append(args, category)
		argN++
	}
	query += fmt.Sprintf(" WHERE ci.status = $%d ORDER BY ci.id", argN)
	args = append(args, StatusPublished)

	var rows []row
	if err := db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("catalog: list: %w", err)
	}
	return attachTags(ctx, db, rows)
}

// listAllCatalogItems returns every catalog item regardless of status,
// for the admin list view.
func listAllCatalogItems(ctx context.Context, db auth.Querier) ([]item, error) {
	var rows []row
	query := `SELECT id, title, marketplace_search_url, image_url, status FROM catalog_items ORDER BY id`
	if err := db.SelectContext(ctx, &rows, query); err != nil {
		return nil, fmt.Errorf("catalog: list all: %w", err)
	}
	return attachTags(ctx, db, rows)
}

// attachTags loads the age-range and category tags for the given rows
// in two batched queries (not one query per item) and bundles each row
// with its tags.
func attachTags(ctx context.Context, db auth.Querier, rows []row) ([]item, error) {
	if len(rows) == 0 {
		return []item{}, nil
	}
	ids := make([]any, len(rows))
	placeholders := make([]string, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	inClause := strings.Join(placeholders, ", ")

	var ageRows []struct {
		CatalogItemID int64  `db:"catalog_item_id"`
		AgeRangeCode  string `db:"age_range_code"`
	}
	ageQuery := fmt.Sprintf(`SELECT catalog_item_id, age_range_code FROM catalog_item_age_ranges WHERE catalog_item_id IN (%s) ORDER BY age_range_code`, inClause)
	if err := db.SelectContext(ctx, &ageRows, ageQuery, ids...); err != nil {
		return nil, fmt.Errorf("catalog: load age ranges: %w", err)
	}
	ageByItem := make(map[int64][]string)
	for _, ar := range ageRows {
		ageByItem[ar.CatalogItemID] = append(ageByItem[ar.CatalogItemID], ar.AgeRangeCode)
	}

	var catRows []struct {
		CatalogItemID int64  `db:"catalog_item_id"`
		Category      string `db:"category"`
	}
	catQuery := fmt.Sprintf(`SELECT catalog_item_id, category FROM catalog_item_categories WHERE catalog_item_id IN (%s) ORDER BY category`, inClause)
	if err := db.SelectContext(ctx, &catRows, catQuery, ids...); err != nil {
		return nil, fmt.Errorf("catalog: load categories: %w", err)
	}
	catByItem := make(map[int64][]string)
	for _, c := range catRows {
		catByItem[c.CatalogItemID] = append(catByItem[c.CatalogItemID], c.Category)
	}

	items := make([]item, len(rows))
	for i, r := range rows {
		items[i] = item{
			ID:                   r.ID,
			Title:                r.Title,
			MarketplaceSearchURL: r.MarketplaceSearchURL,
			ImageURL:             r.ImageURL,
			Status:               r.Status,
			AgeRangeCodes:        ageByItem[r.ID],
			Categories:           catByItem[r.ID],
		}
	}
	return items, nil
}

// getCatalogItem loads a single item with its tags, or errNotFound.
func getCatalogItem(ctx context.Context, db auth.Querier, id int64) (item, error) {
	var r row
	err := db.GetContext(ctx, &r, `SELECT id, title, marketplace_search_url, image_url, status FROM catalog_items WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return item{}, errNotFound
	}
	if err != nil {
		return item{}, fmt.Errorf("catalog: get item %d: %w", id, err)
	}
	items, err := attachTags(ctx, db, []row{r})
	if err != nil {
		return item{}, err
	}
	return items[0], nil
}

// insertTags writes rows into catalog_item_age_ranges and
// catalog_item_categories for itemID. Either slice may be nil to skip
// that table.
func insertTags(ctx context.Context, db auth.Querier, itemID int64, ageRanges, categories []string) error {
	for _, code := range ageRanges {
		if _, err := db.ExecContext(ctx, `INSERT INTO catalog_item_age_ranges (catalog_item_id, age_range_code) VALUES ($1, $2)`, itemID, code); err != nil {
			return fmt.Errorf("catalog: insert age range %q for item %d: %w", code, itemID, err)
		}
	}
	for _, cat := range categories {
		if _, err := db.ExecContext(ctx, `INSERT INTO catalog_item_categories (catalog_item_id, category) VALUES ($1, $2)`, itemID, cat); err != nil {
			return fmt.Errorf("catalog: insert category %q for item %d: %w", cat, itemID, err)
		}
	}
	return nil
}

// nullableImageURL converts an empty string to SQL NULL and any other
// value to a valid NullString — the "no photo" and "clear the photo"
// representations are the same: an empty image_url.
func nullableImageURL(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// createCatalogItem inserts a new catalog item and its tags. These are
// separate statements, not one transaction — auth.Querier only exposes
// GetContext/SelectContext/ExecContext, not a way to begin one (the same
// accepted tradeoff documented on auth.VerifyOTP). If a tag insert fails
// after the item row is created, the item exists with a partial tag
// set — acceptable for this single-admin MVP slice.
func createCatalogItem(ctx context.Context, db auth.Querier, title, url, imageURL string, ageRanges, categories []string, status string) (item, error) {
	var id int64
	err := db.GetContext(ctx, &id, `
		INSERT INTO catalog_items (title, marketplace_search_url, image_url, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id`, title, url, nullableImageURL(imageURL), status)
	if err != nil {
		return item{}, fmt.Errorf("catalog: create item: %w", err)
	}
	if err := insertTags(ctx, db, id, ageRanges, categories); err != nil {
		return item{}, err
	}
	return getCatalogItem(ctx, db, id)
}

// updateCatalogItem partially updates a catalog item: only non-nil
// arguments are changed. A non-nil ageRanges or categories replaces the
// full tag set (delete then insert), not a merge. imageURL follows the
// same empty-string-means-NULL convention as createCatalogItem.
func updateCatalogItem(ctx context.Context, db auth.Querier, id int64, title, url, imageURL, status *string, ageRanges, categories *[]string) (item, error) {
	if _, err := getCatalogItem(ctx, db, id); err != nil {
		return item{}, err
	}

	if title != nil {
		if _, err := db.ExecContext(ctx, `UPDATE catalog_items SET title = $1 WHERE id = $2`, *title, id); err != nil {
			return item{}, fmt.Errorf("catalog: update title for item %d: %w", id, err)
		}
	}
	if url != nil {
		if _, err := db.ExecContext(ctx, `UPDATE catalog_items SET marketplace_search_url = $1 WHERE id = $2`, *url, id); err != nil {
			return item{}, fmt.Errorf("catalog: update url for item %d: %w", id, err)
		}
	}
	if imageURL != nil {
		if _, err := db.ExecContext(ctx, `UPDATE catalog_items SET image_url = $1 WHERE id = $2`, nullableImageURL(*imageURL), id); err != nil {
			return item{}, fmt.Errorf("catalog: update image_url for item %d: %w", id, err)
		}
	}
	if status != nil {
		if _, err := db.ExecContext(ctx, `UPDATE catalog_items SET status = $1 WHERE id = $2`, *status, id); err != nil {
			return item{}, fmt.Errorf("catalog: update status for item %d: %w", id, err)
		}
	}
	if ageRanges != nil {
		if _, err := db.ExecContext(ctx, `DELETE FROM catalog_item_age_ranges WHERE catalog_item_id = $1`, id); err != nil {
			return item{}, fmt.Errorf("catalog: clear age ranges for item %d: %w", id, err)
		}
		if err := insertTags(ctx, db, id, *ageRanges, nil); err != nil {
			return item{}, err
		}
	}
	if categories != nil {
		if _, err := db.ExecContext(ctx, `DELETE FROM catalog_item_categories WHERE catalog_item_id = $1`, id); err != nil {
			return item{}, fmt.Errorf("catalog: clear categories for item %d: %w", id, err)
		}
		if err := insertTags(ctx, db, id, nil, *categories); err != nil {
			return item{}, err
		}
	}

	return getCatalogItem(ctx, db, id)
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/catalog/... -v`
Expected: PASS — every test in `internal/catalog` (this task doesn't touch `response.go`/`handler.go`/their tests, which won't compile yet — run just this package's non-handler tests if the package fails to build: `go test ./internal/catalog/... -run 'TestIsValidStatus|TestValidate|TestListCatalogItems|TestListAllCatalogItems|TestCreateCatalogItem|TestUpdateCatalogItem' -v`; the full package (including `handler_test.go`, `category_test.go`) is verified together at the end of Task 3).

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/status.go internal/catalog/status_test.go \
        internal/catalog/validate.go internal/catalog/validate_test.go \
        internal/catalog/catalog.go internal/catalog/catalog_test.go
git commit -m "Rewrite catalog repository for multi-tag items and status"
```

---

## Task 3: Response shapes and HTTP handlers (public + admin)

**Files:**
- Modify: `backend/internal/catalog/response.go` (full replacement)
- Modify: `backend/internal/catalog/handler.go` (full replacement)
- Modify: `backend/internal/catalog/handler_test.go` (full replacement)

**Interfaces:**
- Consumes: `item`, `listCatalogItems`, `listAllCatalogItems`, `createCatalogItem`, `updateCatalogItem`, `errNotFound`, `IsValidStatus`, `validateTitle`, `validateURL`, `validateAgeRangeCodes`, `validateCategories` (Task 2).
- Produces: `Handler.List` (public, unchanged route), `Handler.ListAdmin`, `Handler.Create`, `Handler.Update` (new). Task 5 registers all four on the router.

- [ ] **Step 1: Write the failing tests**

Replace `backend/internal/catalog/handler_test.go` entirely with:

```go
package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"needtobuy/internal/dbtest"
)

func jsonBody(s string) *strings.Reader { return strings.NewReader(s) }

func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestList_NoFilters_ReturnsPublishedSeedData(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected seeded catalog items, got none")
	}
	for _, item := range items {
		if _, hasStatus := item["status"]; hasStatus {
			t.Fatalf("public response item %v exposes status, want it omitted", item)
		}
		if _, ok := item["age_range_codes"].([]any); !ok {
			t.Fatalf("item %v missing age_range_codes array", item)
		}
		if _, ok := item["categories"].([]any); !ok {
			t.Fatalf("item %v missing categories array", item)
		}
	}
}

func TestList_InvalidAgeRange_BadRequest(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog?age_range=not-a-code", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestList_InvalidCategory_BadRequest(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog?category=shoes", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestListAdmin_IncludesStatusAndHiddenItems(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/catalog",
		jsonBody(`{"title":"Скрытый","marketplace_search_url":"https://example.com","image_url":"","age_range_codes":["18m"],"categories":["toys"],"status":"hidden"}`))
	createRec := httptest.NewRecorder()
	h.Create(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	hiddenID := int64(created["id"].(float64))

	listReq := httptest.NewRequest(http.MethodGet, "/api/admin/catalog", nil)
	listRec := httptest.NewRecorder()
	h.ListAdmin(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list admin: status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	var items []map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	found := false
	for _, item := range items {
		if int64(item["id"].(float64)) == hiddenID {
			found = true
			if item["status"] != "hidden" {
				t.Fatalf("item status = %v, want \"hidden\"", item["status"])
			}
		}
	}
	if !found {
		t.Fatalf("admin list did not include hidden item %d", hiddenID)
	}
}

func TestCreate_ValidBody_Returns201(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/catalog",
		jsonBody(`{"title":"Новый товар","marketplace_search_url":"https://ozon.ru/search/?text=x","image_url":"","age_range_codes":["12m","18m"],"categories":["books","toys"],"status":"published"}`))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["title"] != "Новый товар" {
		t.Fatalf("title = %v, want %q", body["title"], "Новый товар")
	}
	ageRanges, _ := body["age_range_codes"].([]any)
	if len(ageRanges) != 2 {
		t.Fatalf("age_range_codes = %v, want 2 entries", body["age_range_codes"])
	}
}

func TestCreate_EmptyAgeRangeCodes_BadRequest(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/catalog",
		jsonBody(`{"title":"x","marketplace_search_url":"https://example.com","age_range_codes":[],"categories":["toys"],"status":"published"}`))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreate_UnknownCategory_BadRequest(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/catalog",
		jsonBody(`{"title":"x","marketplace_search_url":"https://example.com","age_range_codes":["18m"],"categories":["shoes"],"status":"published"}`))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreate_InvalidStatus_BadRequest(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodPost, "/api/admin/catalog",
		jsonBody(`{"title":"x","marketplace_search_url":"https://example.com","age_range_codes":["18m"],"categories":["toys"],"status":"draft"}`))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestUpdate_PartialBody_OnlyChangesGivenFields(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/catalog",
		jsonBody(`{"title":"Исходный","marketplace_search_url":"https://example.com","age_range_codes":["18m"],"categories":["toys"],"status":"published"}`))
	createRec := httptest.NewRecorder()
	h.Create(createRec, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	id := int64(created["id"].(float64))

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/admin/catalog/"+strconv.FormatInt(id, 10),
		jsonBody(`{"status":"hidden"}`))
	patchReq = withURLParam(patchReq, "id", strconv.FormatInt(id, 10))
	patchRec := httptest.NewRecorder()
	h.Update(patchRec, patchReq)

	if patchRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", patchRec.Code, http.StatusOK, patchRec.Body.String())
	}
	var updated map[string]any
	if err := json.Unmarshal(patchRec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated["status"] != "hidden" {
		t.Fatalf("status = %v, want \"hidden\"", updated["status"])
	}
	if updated["title"] != "Исходный" {
		t.Fatalf("title = %v, want unchanged \"Исходный\"", updated["title"])
	}
}

func TestUpdate_UnknownID_NotFound(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodPatch, "/api/admin/catalog/999999999", jsonBody(`{"status":"hidden"}`))
	req = withURLParam(req, "id", "999999999")
	rec := httptest.NewRecorder()
	h.Update(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/catalog/... -v`
Expected: FAIL — `h.ListAdmin`/`h.Create`/`h.Update` undefined (the test file itself compiles cleanly; only the handler methods it calls don't exist yet).

- [ ] **Step 3: Implement `response.go` and `handler.go`**

Replace `backend/internal/catalog/response.go` entirely with:

```go
package catalog

// itemResponse is the public JSON shape returned by GET /api/catalog.
// AgeRangeCodes/Categories are never null — an item always has at least
// one of each (enforced by validateAgeRangeCodes/validateCategories at
// write time) and toResponse normalizes a nil slice to an empty array
// regardless.
type itemResponse struct {
	ID                   int64    `json:"id"`
	Title                string   `json:"title"`
	MarketplaceSearchURL string   `json:"marketplace_search_url"`
	ImageURL             *string  `json:"image_url"`
	AgeRangeCodes        []string `json:"age_range_codes"`
	Categories           []string `json:"categories"`
}

// adminItemResponse extends itemResponse with the status field, which
// the public endpoint never exposes.
type adminItemResponse struct {
	itemResponse
	Status string `json:"status"`
}

func toResponse(it item) itemResponse {
	resp := itemResponse{
		ID:                   it.ID,
		Title:                it.Title,
		MarketplaceSearchURL: it.MarketplaceSearchURL,
		AgeRangeCodes:        it.AgeRangeCodes,
		Categories:           it.Categories,
	}
	if it.ImageURL.Valid {
		resp.ImageURL = &it.ImageURL.String
	}
	if resp.AgeRangeCodes == nil {
		resp.AgeRangeCodes = []string{}
	}
	if resp.Categories == nil {
		resp.Categories = []string{}
	}
	return resp
}

func toAdminResponse(it item) adminItemResponse {
	return adminItemResponse{itemResponse: toResponse(it), Status: it.Status}
}
```

Replace `backend/internal/catalog/handler.go` entirely with:

```go
package catalog

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"needtobuy/internal/agerange"
	"needtobuy/internal/apierr"
	"needtobuy/internal/auth"
)

// Handler wires the catalog HTTP endpoints (public browsing and admin
// moderation) to a database.
type Handler struct {
	db auth.Querier
}

// NewHandler builds a Handler ready to register on a router.
func NewHandler(database auth.Querier) *Handler {
	return &Handler{db: database}
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// List handles GET /api/catalog. No authentication is required — the
// catalog is a public reference, not tied to any parent's account. Only
// published items are returned.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ageRange := r.URL.Query().Get("age_range")
	if ageRange != "" && !agerange.IsValid(ageRange) {
		apierr.WriteError(w, apierr.BadRequest("age_range is not a known age bucket"))
		return
	}
	category := r.URL.Query().Get("category")
	if category != "" && !IsValidCategory(category) {
		apierr.WriteError(w, apierr.BadRequest("category is not a known category"))
		return
	}

	items, err := listCatalogItems(r.Context(), h.db, ageRange, category)
	if err != nil {
		log.Printf("catalog: list age_range=%q category=%q: %v", ageRange, category, err)
		apierr.WriteError(w, apierr.Internal("could not load catalog"))
		return
	}
	responses := make([]itemResponse, len(items))
	for i, it := range items {
		responses[i] = toResponse(it)
	}
	apierr.WriteJSON(w, http.StatusOK, responses)
}

// ListAdmin handles GET /api/admin/catalog. Must be registered behind
// auth.RequireAuth + authHandler.RequireAdmin.
func (h *Handler) ListAdmin(w http.ResponseWriter, r *http.Request) {
	items, err := listAllCatalogItems(r.Context(), h.db)
	if err != nil {
		log.Printf("catalog: list all: %v", err)
		apierr.WriteError(w, apierr.Internal("could not load catalog"))
		return
	}
	responses := make([]adminItemResponse, len(items))
	for i, it := range items {
		responses[i] = toAdminResponse(it)
	}
	apierr.WriteJSON(w, http.StatusOK, responses)
}

type adminCreateBody struct {
	Title                 string   `json:"title"`
	MarketplaceSearchURL string   `json:"marketplace_search_url"`
	ImageURL              string   `json:"image_url"`
	AgeRangeCodes         []string `json:"age_range_codes"`
	Categories            []string `json:"categories"`
	Status                string   `json:"status"`
}

// Create handles POST /api/admin/catalog. Must be registered behind
// auth.RequireAuth + authHandler.RequireAdmin.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var body adminCreateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierr.WriteError(w, apierr.BadRequest("invalid request body"))
		return
	}
	title, err := validateTitle(body.Title)
	if err != nil {
		apierr.WriteError(w, apierr.BadRequest(err.Error()))
		return
	}
	url, err := validateURL(body.MarketplaceSearchURL)
	if err != nil {
		apierr.WriteError(w, apierr.BadRequest(err.Error()))
		return
	}
	if err := validateAgeRangeCodes(body.AgeRangeCodes); err != nil {
		apierr.WriteError(w, apierr.BadRequest(err.Error()))
		return
	}
	if err := validateCategories(body.Categories); err != nil {
		apierr.WriteError(w, apierr.BadRequest(err.Error()))
		return
	}
	if !IsValidStatus(body.Status) {
		apierr.WriteError(w, apierr.BadRequest(`status must be "published" or "hidden"`))
		return
	}

	created, err := createCatalogItem(r.Context(), h.db, title, url, body.ImageURL, body.AgeRangeCodes, body.Categories, body.Status)
	if err != nil {
		log.Printf("catalog: create item: %v", err)
		apierr.WriteError(w, apierr.Internal("could not create catalog item"))
		return
	}
	apierr.WriteJSON(w, http.StatusCreated, toAdminResponse(created))
}

type adminUpdateBody struct {
	Title                 *string   `json:"title"`
	MarketplaceSearchURL *string   `json:"marketplace_search_url"`
	ImageURL              *string   `json:"image_url"`
	AgeRangeCodes         *[]string `json:"age_range_codes"`
	Categories            *[]string `json:"categories"`
	Status                *string   `json:"status"`
}

// Update handles PATCH /api/admin/catalog/{id}. Must be registered
// behind auth.RequireAuth + authHandler.RequireAdmin. Any field present
// in the JSON body replaces its current value entirely; an absent field
// is left untouched.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		apierr.WriteError(w, apierr.BadRequest("invalid catalog item id"))
		return
	}
	var body adminUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierr.WriteError(w, apierr.BadRequest("invalid request body"))
		return
	}

	var titlePtr *string
	if body.Title != nil {
		title, err := validateTitle(*body.Title)
		if err != nil {
			apierr.WriteError(w, apierr.BadRequest(err.Error()))
			return
		}
		titlePtr = &title
	}
	var urlPtr *string
	if body.MarketplaceSearchURL != nil {
		url, err := validateURL(*body.MarketplaceSearchURL)
		if err != nil {
			apierr.WriteError(w, apierr.BadRequest(err.Error()))
			return
		}
		urlPtr = &url
	}
	if body.AgeRangeCodes != nil {
		if err := validateAgeRangeCodes(*body.AgeRangeCodes); err != nil {
			apierr.WriteError(w, apierr.BadRequest(err.Error()))
			return
		}
	}
	if body.Categories != nil {
		if err := validateCategories(*body.Categories); err != nil {
			apierr.WriteError(w, apierr.BadRequest(err.Error()))
			return
		}
	}
	if body.Status != nil && !IsValidStatus(*body.Status) {
		apierr.WriteError(w, apierr.BadRequest(`status must be "published" or "hidden"`))
		return
	}

	updated, err := updateCatalogItem(r.Context(), h.db, id, titlePtr, urlPtr, body.ImageURL, body.Status, body.AgeRangeCodes, body.Categories)
	if errors.Is(err, errNotFound) {
		apierr.WriteError(w, apierr.NotFound("catalog item"))
		return
	}
	if err != nil {
		log.Printf("catalog: update item %d: %v", id, err)
		apierr.WriteError(w, apierr.Internal("could not update catalog item"))
		return
	}
	apierr.WriteJSON(w, http.StatusOK, toAdminResponse(updated))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/catalog/... -v`
Expected: PASS — every test in `internal/catalog` (`status_test.go`, `validate_test.go`, `catalog_test.go`, `category_test.go`, `handler_test.go`).

- [ ] **Step 5: Commit**

```bash
git add internal/catalog/response.go internal/catalog/handler.go internal/catalog/handler_test.go
git commit -m "Add catalog admin HTTP handlers (list, create, update)"
```

---

## Task 4: Admin gating in auth — `RequireAdmin`, `is_admin`, config

**Files:**
- Modify: `backend/internal/config/config.go`
- Modify: `backend/internal/config/config_test.go`
- Modify: `backend/internal/auth/handler.go`
- Modify: `backend/internal/auth/handler_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `config.Config.AdminEmail`; `auth.NewHandler(database Querier, mailer Mailer, pepper, adminEmail string) *Handler` (signature grows a 4th param); `(*Handler).RequireAdmin(next http.Handler) http.Handler`; `GET /api/auth/me` response gains `is_admin bool`. Task 5's router wiring and every existing `auth.NewHandler(...)` call site consume the new signature.

- [ ] **Step 1: Write the failing tests**

Append to `backend/internal/config/config_test.go`, inside `TestLoad_Defaults` (add these lines right after the `OTPPepper` check, before the closing brace) and inside `TestLoad_FromEnv` (same placement) — since this is an in-place edit to two existing functions, here is the complete replacement for `backend/internal/config/config_test.go`:

```go
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
	t.Setenv("ADMIN_EMAIL", "")

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
	if cfg.AdminEmail != "babaliants@gmail.com" {
		t.Errorf("AdminEmail = %q, want %q", cfg.AdminEmail, "babaliants@gmail.com")
	}
}

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://custom/db")
	t.Setenv("PORT", "9090")
	t.Setenv("SMTP_ADDR", "smtp.example.com:587")
	t.Setenv("SMTP_FROM", "hello@needtobuy.ru")
	t.Setenv("OTP_PEPPER", "prod-secret")
	t.Setenv("ADMIN_EMAIL", "admin@needtobuy.ru")

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
	if cfg.AdminEmail != "admin@needtobuy.ru" {
		t.Errorf("AdminEmail = %q, want %q", cfg.AdminEmail, "admin@needtobuy.ru")
	}
}
```

Append to `backend/internal/auth/handler_test.go` (add these test functions at the end of the file):

```go
func TestMe_ValidSession_IncludesIsAdminFalseForNonAdmin(t *testing.T) {
	tx := dbtest.Tx(t)
	mailer := &capturingMailer{}
	h := NewHandler(tx, mailer, "pepper", "admin@needtobuy.ru")
	protected := h.Middleware(RequireAuth(http.HandlerFunc(h.Me)))

	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"parent@example.com"}`))
	h.RequestOTP(httptest.NewRecorder(), reqReq)
	verifyBody := `{"email":"parent@example.com","code":"` + mailer.lastCode + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	h.VerifyOTP(verifyRec, verifyReq)
	sessionCookieValue := verifyRec.Result().Cookies()[0]

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(sessionCookieValue)
	meRec := httptest.NewRecorder()
	protected.ServeHTTP(meRec, meReq)

	var body map[string]any
	if err := json.Unmarshal(meRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["is_admin"] != false {
		t.Fatalf("is_admin = %v, want false", body["is_admin"])
	}
}

func TestMe_ValidSession_IncludesIsAdminTrueForAdmin(t *testing.T) {
	tx := dbtest.Tx(t)
	mailer := &capturingMailer{}
	h := NewHandler(tx, mailer, "pepper", "admin@needtobuy.ru")
	protected := h.Middleware(RequireAuth(http.HandlerFunc(h.Me)))

	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"admin@needtobuy.ru"}`))
	h.RequestOTP(httptest.NewRecorder(), reqReq)
	verifyBody := `{"email":"admin@needtobuy.ru","code":"` + mailer.lastCode + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	h.VerifyOTP(verifyRec, verifyReq)
	sessionCookieValue := verifyRec.Result().Cookies()[0]

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(sessionCookieValue)
	meRec := httptest.NewRecorder()
	protected.ServeHTTP(meRec, meReq)

	var body map[string]any
	if err := json.Unmarshal(meRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["is_admin"] != true {
		t.Fatalf("is_admin = %v, want true", body["is_admin"])
	}
}

func TestRequireAdmin_NoSession_Forbidden(t *testing.T) {
	tx := dbtest.Tx(t)
	mailer := &capturingMailer{}
	h := NewHandler(tx, mailer, "pepper", "admin@needtobuy.ru")
	protected := h.Middleware(h.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	req := httptest.NewRequest(http.MethodGet, "/api/admin/catalog", nil)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireAdmin_NonAdminSession_Forbidden(t *testing.T) {
	tx := dbtest.Tx(t)
	mailer := &capturingMailer{}
	h := NewHandler(tx, mailer, "pepper", "admin@needtobuy.ru")
	protected := h.Middleware(h.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"parent@example.com"}`))
	h.RequestOTP(httptest.NewRecorder(), reqReq)
	verifyBody := `{"email":"parent@example.com","code":"` + mailer.lastCode + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	h.VerifyOTP(verifyRec, verifyReq)
	sessionCookieValue := verifyRec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/api/admin/catalog", nil)
	req.AddCookie(sessionCookieValue)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireAdmin_AdminSession_Allowed(t *testing.T) {
	tx := dbtest.Tx(t)
	mailer := &capturingMailer{}
	h := NewHandler(tx, mailer, "pepper", "admin@needtobuy.ru")
	protected := h.Middleware(h.RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"admin@needtobuy.ru"}`))
	h.RequestOTP(httptest.NewRecorder(), reqReq)
	verifyBody := `{"email":"admin@needtobuy.ru","code":"` + mailer.lastCode + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	h.VerifyOTP(verifyRec, verifyReq)
	sessionCookieValue := verifyRec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/api/admin/catalog", nil)
	req.AddCookie(sessionCookieValue)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
```

This file's `newTestHandler` helper (used by every other existing test) also needs updating to pass a 4th argument — and `TestMe_ValidSession_ReturnsEmail`'s `map[string]string` decode target will break once `is_admin` (a JSON bool) is added to the response, since `map[string]string` can't hold a bool value. Both fixes are in Step 3 below.

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/config/... ./internal/auth/... -v`
Expected: FAIL — `not enough arguments in call to NewHandler`, `undefined: h.RequireAdmin`, `cfg.AdminEmail undefined`.

- [ ] **Step 3: Implement the config, auth, and fix the pre-existing tests**

Replace `backend/internal/config/config.go` entirely with:

```go
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
	AdminEmail  string // the single account allowed to use /api/admin/*
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
		AdminEmail:  getenv("ADMIN_EMAIL", "babaliants@gmail.com"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

In `backend/internal/auth/handler.go`:

1. Change the `Handler` struct and `NewHandler`:

```go
// Handler wires the OTP and session HTTP endpoints to a database and
// mailer.
type Handler struct {
	db         Querier
	mailer     Mailer
	pepper     string
	adminEmail string
}

// NewHandler builds a Handler ready to register on a router. adminEmail
// is the single account RequireAdmin allows through — see
// docs/superpowers/specs/2026-08-22-catalog-admin-design.md.
func NewHandler(database Querier, mailer Mailer, pepper, adminEmail string) *Handler {
	return &Handler{db: database, mailer: mailer, pepper: pepper, adminEmail: adminEmail}
}
```

2. Replace the `Me` method:

```go
// Me handles GET /api/auth/me. It must be registered behind RequireAuth —
// it reports 401 defensively if that invariant is ever violated.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserID(r.Context())
	if !ok {
		apierr.WriteError(w, unauthorized("login required"))
		return
	}
	email, err := emailByUserID(r.Context(), h.db, userID)
	if err != nil {
		log.Printf("auth: lookup email for user %d: %v", userID, err)
		apierr.WriteError(w, apierr.Internal("could not load account"))
		return
	}
	apierr.WriteJSON(w, http.StatusOK, map[string]any{"email": email, "is_admin": email == h.adminEmail})
}
```

3. Add `RequireAdmin` and `forbidden` right after the existing `RequireAuth` function:

```go
// RequireAdmin wraps next so it responds 403 unless the session's email
// matches h.adminEmail. Chain it after RequireAuth (or after Middleware
// on a route already gated some other way) so UserID is already in the
// context — a request with no user id is treated as non-admin (403), not
// escalated to a 401, since every real /api/admin/* registration also
// carries RequireAuth ahead of this middleware.
func (h *Handler) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, ok := UserID(r.Context())
		if !ok {
			apierr.WriteError(w, forbidden("admin access required"))
			return
		}
		email, err := emailByUserID(r.Context(), h.db, userID)
		if err != nil {
			log.Printf("auth: lookup email for admin check, user %d: %v", userID, err)
			apierr.WriteError(w, apierr.Internal("could not verify admin access"))
			return
		}
		if email != h.adminEmail {
			apierr.WriteError(w, forbidden("admin access required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func forbidden(message string) *apierr.Error {
	return &apierr.Error{Code: "forbidden", Message: message, HTTPStatus: http.StatusForbidden}
}
```

In `backend/internal/auth/handler_test.go`, fix the two pre-existing call sites and the one pre-existing type mismatch:

- `newTestHandler` currently does `return NewHandler(dbtest.Tx(t), mailer, "pepper"), mailer` — change to `return NewHandler(dbtest.Tx(t), mailer, "pepper", "admin@needtobuy.ru"), mailer`.
- `TestMe_ValidSession_ReturnsEmail` currently does `var body map[string]string` — change to `var body map[string]any`, and its assertion `if body["email"] != "parent@example.com"` still works unchanged (a `map[string]any` holding a string value compares equal to a string literal the same way).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/config/... ./internal/auth/... -v`
Expected: PASS — every test in both packages.

- [ ] **Step 5: Commit**

```bash
git add internal/config internal/auth
git commit -m "Add admin email config, RequireAdmin middleware, is_admin in /me"
```

---

## Task 5: Wire admin routes into the router; end-to-end tests; main.go; docker-compose

**Files:**
- Modify: `backend/internal/httpapi/router.go` (full replacement)
- Modify: `backend/internal/httpapi/router_test.go` (full replacement)
- Modify: `backend/internal/httpapi/auth_flow_test.go` (full replacement)
- Modify: `backend/internal/httpapi/catalog_flow_test.go` (full replacement)
- Modify: `backend/internal/httpapi/child_flow_test.go` (full replacement)
- Create: `backend/internal/httpapi/catalog_admin_flow_test.go`
- Modify: `backend/internal/child/handler_test.go`
- Modify: `backend/cmd/server/main.go`
- Modify: `docker-compose.yml`

**Interfaces:**
- Consumes: `catalogHandler.ListAdmin/Create/Update` (Task 3), `authHandler.RequireAdmin` (Task 4), `auth.NewHandler`'s 4-arg signature (Task 4).
- Produces: fully wired `/api/admin/catalog` routes. Nothing later in this plan depends on router internals directly (frontend tasks only depend on the HTTP contract, already fixed by Tasks 3-4).

Every call site of `auth.NewHandler` across this package and `internal/child` needs its new 4th argument. Full replacement content is given for every touched file.

- [ ] **Step 1: Write the failing end-to-end test**

`backend/internal/httpapi/catalog_admin_flow_test.go`:

```go
package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"needtobuy/internal/auth"
	"needtobuy/internal/catalog"
	"needtobuy/internal/child"
	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
	"needtobuy/internal/httpapi"
)

func TestCatalogAdminFlow_GatingAndCRUD_EndToEnd(t *testing.T) {
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
	authHandler := auth.NewHandler(conn, mailer, "pepper", "admin@needtobuy.ru")
	childHandler := child.NewHandler(conn)
	catalogHandler := catalog.NewHandler(conn)
	router := httpapi.NewRouter(conn, authHandler, childHandler, catalogHandler)

	// No session at all: 401.
	noCookieReq := httptest.NewRequest(http.MethodGet, "/api/admin/catalog", nil)
	noCookieRec := httptest.NewRecorder()
	router.ServeHTTP(noCookieRec, noCookieReq)
	if noCookieRec.Code != http.StatusUnauthorized {
		t.Fatalf("admin list without session: status = %d, want %d", noCookieRec.Code, http.StatusUnauthorized)
	}

	// Log in as a non-admin parent.
	parentEmail := fmt.Sprintf("parent-%d@example.com", time.Now().UnixNano())
	parentCookie := loginAndGetCookie(t, router, mailer, parentEmail)

	// Non-admin session: 403.
	nonAdminReq := httptest.NewRequest(http.MethodGet, "/api/admin/catalog", nil)
	nonAdminReq.AddCookie(parentCookie)
	nonAdminRec := httptest.NewRecorder()
	router.ServeHTTP(nonAdminRec, nonAdminReq)
	if nonAdminRec.Code != http.StatusForbidden {
		t.Fatalf("admin list with non-admin session: status = %d, want %d", nonAdminRec.Code, http.StatusForbidden)
	}

	// Log in as the admin.
	adminCookie := loginAndGetCookie(t, router, mailer, "admin@needtobuy.ru")

	// Admin session: create a hidden item.
	createBody := `{"title":"E2E товар","marketplace_search_url":"https://example.com","image_url":"",` +
		`"age_range_codes":["12m","18m"],"categories":["books","toys"],"status":"hidden"}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/admin/catalog", strings.NewReader(createBody))
	createReq.AddCookie(adminCookie)
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

	// Hidden item must NOT appear on the public, unauthenticated catalog.
	publicReq := httptest.NewRequest(http.MethodGet, "/api/catalog", nil)
	publicRec := httptest.NewRecorder()
	router.ServeHTTP(publicRec, publicReq)
	var publicItems []map[string]any
	if err := json.Unmarshal(publicRec.Body.Bytes(), &publicItems); err != nil {
		t.Fatalf("unmarshal public catalog: %v", err)
	}
	for _, item := range publicItems {
		if int64(item["id"].(float64)) == id {
			t.Fatalf("hidden item %d leaked into the public catalog", id)
		}
	}

	// Publish it via PATCH.
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/admin/catalog/"+strconv.FormatInt(id, 10),
		strings.NewReader(`{"status":"published"}`))
	patchReq.AddCookie(adminCookie)
	patchRec := httptest.NewRecorder()
	router.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch: status = %d, body = %s", patchRec.Code, patchRec.Body.String())
	}

	// Now it appears on the public catalog.
	publicReq2 := httptest.NewRequest(http.MethodGet, "/api/catalog", nil)
	publicRec2 := httptest.NewRecorder()
	router.ServeHTTP(publicRec2, publicReq2)
	var publicItems2 []map[string]any
	if err := json.Unmarshal(publicRec2.Body.Bytes(), &publicItems2); err != nil {
		t.Fatalf("unmarshal public catalog: %v", err)
	}
	found := false
	for _, item := range publicItems2 {
		if int64(item["id"].(float64)) == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("published item %d did not appear in the public catalog", id)
	}
}

// loginAndGetCookie drives the OTP request/verify flow through router and
// returns the resulting session cookie.
func loginAndGetCookie(t *testing.T, router http.Handler, mailer *capturingMailer, email string) *http.Cookie {
	t.Helper()
	reqBody := fmt.Sprintf(`{"email":%q}`, email)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("otp request for %s: status = %d, body = %s", email, rec.Code, rec.Body.String())
	}
	verifyBody := fmt.Sprintf(`{"email":%q,"code":%q}`, email, mailer.lastCode)
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	router.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("otp verify for %s: status = %d, body = %s", email, verifyRec.Code, verifyRec.Body.String())
	}
	return verifyRec.Result().Cookies()[0]
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./internal/httpapi/... -v`
Expected: FAIL — every existing call to `auth.NewHandler` in this package is missing its new 4th argument, so the whole test binary fails to compile.

- [ ] **Step 3: Update `router.go`**

Replace `backend/internal/httpapi/router.go` entirely with:

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
	"needtobuy/internal/catalog"
	"needtobuy/internal/child"
)

// NewRouter builds the top-level chi router. database is used by the
// health check to verify connectivity; authHandler registers the OTP,
// logout, and me endpoints and its Middleware runs on every request so
// downstream handlers can read the authenticated parent via
// auth.UserID; childHandler registers the child-profile CRUD endpoints
// behind auth.RequireAuth; catalogHandler registers the public catalog
// browsing endpoint (no authentication required, same as /healthz) and
// the admin moderation endpoints behind auth.RequireAuth +
// authHandler.RequireAdmin.
func NewRouter(database *sqlx.DB, authHandler *auth.Handler, childHandler *child.Handler, catalogHandler *catalog.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(authHandler.Middleware)

	r.Get("/healthz", healthHandler(database))
	r.Get("/api/catalog", catalogHandler.List)

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

	r.Route("/api/admin/catalog", func(r chi.Router) {
		r.Use(auth.RequireAuth)
		r.Use(authHandler.RequireAdmin)
		r.Get("/", catalogHandler.ListAdmin)
		r.Post("/", catalogHandler.Create)
		r.Patch("/{id}", catalogHandler.Update)
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

- [ ] **Step 4: Update `router_test.go`**

Replace `backend/internal/httpapi/router_test.go` entirely with:

```go
package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"needtobuy/internal/auth"
	"needtobuy/internal/catalog"
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

	authHandler := auth.NewHandler(conn, noopMailer{}, "test-pepper", "admin@needtobuy.ru")
	childHandler := child.NewHandler(conn)
	catalogHandler := catalog.NewHandler(conn)
	router := httpapi.NewRouter(conn, authHandler, childHandler, catalogHandler)

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
	authHandler := auth.NewHandler(conn, noopMailer{}, "test-pepper", "admin@needtobuy.ru")
	childHandler := child.NewHandler(conn)
	catalogHandler := catalog.NewHandler(conn)
	conn.Close() // force the ping in the handler to fail

	router := httpapi.NewRouter(conn, authHandler, childHandler, catalogHandler)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}
```

- [ ] **Step 5: Update `auth_flow_test.go`**

Replace `backend/internal/httpapi/auth_flow_test.go` entirely with:

```go
package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"needtobuy/internal/auth"
	"needtobuy/internal/catalog"
	"needtobuy/internal/child"
	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
	"needtobuy/internal/httpapi"
)

// capturingMailer records the last code it was asked to send.
//
// This test writes one real, non-transactional user/session row (unlike
// the rest of the suite, which uses dbtest.Tx) because httpapi.NewRouter
// needs a live *sqlx.DB for its health check, not a transaction. The email
// is randomized per run so repeat runs don't collide; the local dev
// database is disposable (`docker compose down -v` resets it).
type capturingMailer struct {
	lastCode string
}

func (m *capturingMailer) SendOTP(_ context.Context, _ string, code string) error {
	m.lastCode = code
	return nil
}

func TestAuthFlow_RequestVerifyLogout_EndToEnd(t *testing.T) {
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
	authHandler := auth.NewHandler(conn, mailer, "pepper", "admin@needtobuy.ru")
	childHandler := child.NewHandler(conn)
	catalogHandler := catalog.NewHandler(conn)
	router := httpapi.NewRouter(conn, authHandler, childHandler, catalogHandler)
	email := fmt.Sprintf("e2e-%d@example.com", time.Now().UnixNano())

	reqBody := fmt.Sprintf(`{"email":%q}`, email)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("request: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	verifyBody := fmt.Sprintf(`{"email":%q,"code":%q}`, email, mailer.lastCode)
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	router.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify: status = %d, body = %s", verifyRec.Code, verifyRec.Body.String())
	}
	cookies := verifyRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("verify: no session cookie set")
	}
	sessionCookie := cookies[0]

	// No cookie at all: /api/auth/me must reject.
	noCookieReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	noCookieRec := httptest.NewRecorder()
	router.ServeHTTP(noCookieRec, noCookieReq)
	if noCookieRec.Code != http.StatusUnauthorized {
		t.Fatalf("me without cookie: status = %d, want %d", noCookieRec.Code, http.StatusUnauthorized)
	}

	// With the real session cookie: /api/auth/me must report the email.
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(sessionCookie)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me with cookie: status = %d, body = %s", meRec.Code, meRec.Body.String())
	}
	var me map[string]any
	if err := json.Unmarshal(meRec.Body.Bytes(), &me); err != nil {
		t.Fatalf("unmarshal me body: %v", err)
	}
	if me["email"] != email {
		t.Fatalf("me email = %v, want %q", me["email"], email)
	}
	if me["is_admin"] != false {
		t.Fatalf("me is_admin = %v, want false", me["is_admin"])
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthReq.AddCookie(sessionCookie)
	healthRec := httptest.NewRecorder()
	router.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("healthz with session cookie: status = %d, body = %s", healthRec.Code, healthRec.Body.String())
	}
	var health map[string]string
	if err := json.Unmarshal(healthRec.Body.Bytes(), &health); err != nil {
		t.Fatalf("unmarshal healthz body: %v", err)
	}
	if health["status"] != "ok" {
		t.Fatalf(`healthz status = %q, want "ok"`, health["status"])
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout: status = %d, body = %s", logoutRec.Code, logoutRec.Body.String())
	}

	// Same session cookie after logout: the session must be revoked.
	postLogoutReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	postLogoutReq.AddCookie(sessionCookie)
	postLogoutRec := httptest.NewRecorder()
	router.ServeHTTP(postLogoutRec, postLogoutReq)
	if postLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: status = %d, want %d", postLogoutRec.Code, http.StatusUnauthorized)
	}
}
```

- [ ] **Step 6: Update `catalog_flow_test.go`**

Replace `backend/internal/httpapi/catalog_flow_test.go` entirely with:

```go
package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"needtobuy/internal/auth"
	"needtobuy/internal/catalog"
	"needtobuy/internal/child"
	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
	"needtobuy/internal/httpapi"
)

func TestCatalogFlow_BrowseWithoutSession_EndToEnd(t *testing.T) {
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
	authHandler := auth.NewHandler(conn, mailer, "pepper", "admin@needtobuy.ru")
	childHandler := child.NewHandler(conn)
	catalogHandler := catalog.NewHandler(conn)
	router := httpapi.NewRouter(conn, authHandler, childHandler, catalogHandler)

	// No session cookie at all: the catalog must still respond 200 — it's
	// the one domain route in this project with no auth.RequireAuth.
	req := httptest.NewRequest(http.MethodGet, "/api/catalog", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog without session: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected seeded catalog items, got none")
	}

	// Filtered request.
	filteredReq := httptest.NewRequest(http.MethodGet, "/api/catalog?age_range=18m&category=toys", nil)
	filteredRec := httptest.NewRecorder()
	router.ServeHTTP(filteredRec, filteredReq)
	if filteredRec.Code != http.StatusOK {
		t.Fatalf("filtered catalog: status = %d, body = %s", filteredRec.Code, filteredRec.Body.String())
	}
	var filtered []map[string]any
	if err := json.Unmarshal(filteredRec.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("unmarshal filtered: %v", err)
	}
	if len(filtered) == 0 {
		t.Fatal("expected at least one seeded item at 18m/toys")
	}

	// Invalid filter: 400, not a silent empty list.
	badReq := httptest.NewRequest(http.MethodGet, "/api/catalog?category=shoes", nil)
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid category: status = %d, want %d", badRec.Code, http.StatusBadRequest)
	}
}
```

- [ ] **Step 7: Update `child_flow_test.go`**

In `backend/internal/httpapi/child_flow_test.go`, the only change is the `auth.NewHandler(conn, mailer, "pepper")` call — replace it with `auth.NewHandler(conn, mailer, "pepper", "admin@needtobuy.ru")`. Every other line is unchanged from the file's current content.

- [ ] **Step 8: Update `internal/child/handler_test.go`**

In `backend/internal/child/handler_test.go`, inside `newTestEnv`, change `authHandler: auth.NewHandler(tx, mailer, "pepper"),` to `authHandler: auth.NewHandler(tx, mailer, "pepper", "admin@needtobuy.ru"),`. Every other line is unchanged.

- [ ] **Step 9: Update `cmd/server/main.go`**

Replace `backend/cmd/server/main.go` entirely with:

```go
// Command server runs the NeedToBuy HTTP API.
package main

import (
	"log"
	"net/http"

	"needtobuy/internal/auth"
	"needtobuy/internal/catalog"
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
	authHandler := auth.NewHandler(database, mailer, cfg.OTPPepper, cfg.AdminEmail)
	childHandler := child.NewHandler(database)
	catalogHandler := catalog.NewHandler(database)

	router := httpapi.NewRouter(database, authHandler, childHandler, catalogHandler)

	log.Printf("listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
```

- [ ] **Step 10: Update `docker-compose.yml`**

In the repo root's `docker-compose.yml`, the `backend` service's `environment:` block currently reads:

```yaml
    environment:
      DATABASE_URL: postgres://needtobuy:needtobuy@postgres:5432/needtobuy?sslmode=disable
      SMTP_ADDR: mailcatcher:1025
      SMTP_FROM: no-reply@needtobuy.local
      OTP_PEPPER: dev-insecure-pepper-change-me
      PORT: "8080"
```

Add one line so it reads:

```yaml
    environment:
      DATABASE_URL: postgres://needtobuy:needtobuy@postgres:5432/needtobuy?sslmode=disable
      SMTP_ADDR: mailcatcher:1025
      SMTP_FROM: no-reply@needtobuy.local
      OTP_PEPPER: dev-insecure-pepper-change-me
      ADMIN_EMAIL: babaliants@gmail.com
      PORT: "8080"
```

Every other line in the file is unchanged.

- [ ] **Step 11: Run the full backend test suite**

Run: `cd backend && TEST_DATABASE_URL="postgres://needtobuy:needtobuy@localhost:5432/needtobuy?sslmode=disable" go test ./... -v`
Expected: PASS — every package.

- [ ] **Step 12: Verify the server builds**

Run (from `backend/`): `go build ./... && echo BUILD_OK`
Expected: `BUILD_OK`.

- [ ] **Step 13: Commit**

```bash
git add internal/httpapi internal/child/handler_test.go cmd/server docker-compose.yml
git commit -m "Wire catalog admin endpoints into the router"
```

---

## Task 6: Frontend — multi-tag `CatalogItem` and updated public card

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/client.test.ts`
- Create: `frontend/src/catalog/categories.ts`
- Create: `frontend/src/catalog/categories.test.ts`
- Modify: `frontend/src/catalog/CatalogPage.tsx`
- Modify: `frontend/src/catalog/CatalogPage.test.tsx`

**Interfaces:**
- Produces: `CatalogItem { id, title, marketplace_search_url, image_url, age_range_codes: string[], categories: string[] }` (replaces the old singular-field shape); `CATEGORIES: {value, label}[]` in `categories.ts`. Task 9 (admin page) consumes `CATEGORIES`.

- [ ] **Step 1: Write the failing tests**

`frontend/src/catalog/categories.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { CATEGORIES } from './categories'

describe('CATEGORIES', () => {
  it('has exactly the 4 known category values', () => {
    const values = CATEGORIES.map((c) => c.value)
    expect(values).toEqual(['clothes', 'toys', 'books', 'sport'])
  })
})
```

Replace the top of `frontend/src/catalog/CatalogPage.test.tsx` (the `SEED_ITEM`/`SEED_ITEM_WITH_IMAGE` constants) with the new multi-tag shape — full replacement of `frontend/src/catalog/CatalogPage.test.tsx`:

```tsx
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { CatalogPage } from './CatalogPage'
import * as client from '../api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

const SEED_ITEM = {
  id: 1,
  age_range_codes: ['18m'],
  categories: ['toys'],
  title: 'Сортер с крупными деталями',
  marketplace_search_url: 'https://www.ozon.ru/search/?text=сортер',
  image_url: null,
}

const SEED_ITEM_MULTI_TAG = {
  id: 3,
  age_range_codes: ['12m', '18m'],
  categories: ['books', 'toys'],
  title: 'Книжка-непромокашка',
  marketplace_search_url: 'https://www.ozon.ru/search/?text=книжка',
  image_url: null,
}

const SEED_ITEM_WITH_IMAGE = {
  ...SEED_ITEM,
  id: 2,
  title: 'Пирамидка с крупными кольцами',
  image_url: 'https://example.com/pyramid.jpg',
}

describe('CatalogPage', () => {
  it('renders items returned by getCatalog', async () => {
    vi.spyOn(client, 'getCatalog').mockResolvedValue([SEED_ITEM])
    render(<CatalogPage />)
    await waitFor(() => expect(screen.getByText('Сортер с крупными деталями')).toBeInTheDocument())
  })

  it('renders a tag for every age range and category the item has', async () => {
    vi.spyOn(client, 'getCatalog').mockResolvedValue([SEED_ITEM_MULTI_TAG])
    render(<CatalogPage />)
    await waitFor(() => expect(screen.getByText('Книжка-непромокашка')).toBeInTheDocument())
    expect(screen.getByText('12m')).toBeInTheDocument()
    expect(screen.getByText('18m')).toBeInTheDocument()
    expect(screen.getByText('Книги')).toBeInTheDocument()
    expect(screen.getByText('Игрушки')).toBeInTheDocument()
  })

  it('shows the empty-state message when there are no results', async () => {
    vi.spyOn(client, 'getCatalog').mockResolvedValue([])
    render(<CatalogPage />)
    await waitFor(() =>
      expect(
        screen.getByText('Пока нет идей для этого возраста и категории — попробуйте другой фильтр.'),
      ).toBeInTheDocument(),
    )
  })

  it('shows the server error message when the request fails', async () => {
    vi.spyOn(client, 'getCatalog').mockRejectedValue(
      new client.ApiError('bad_request', 'category is not a known category', 400),
    )
    render(<CatalogPage />)
    await waitFor(() => expect(screen.getByText('category is not a known category')).toBeInTheDocument())
  })

  it('re-fetches with the new category when the filter changes', async () => {
    const getCatalog = vi.spyOn(client, 'getCatalog').mockResolvedValue([SEED_ITEM])
    const user = userEvent.setup()
    render(<CatalogPage />)
    await waitFor(() => expect(getCatalog).toHaveBeenCalledWith({ ageRange: undefined, category: undefined }))

    await user.click(screen.getByRole('radio', { name: 'Игрушки' }))

    await waitFor(() => expect(getCatalog).toHaveBeenCalledWith({ ageRange: undefined, category: 'toys' }))
  })

  it('renders a photo when the item has an image_url', async () => {
    vi.spyOn(client, 'getCatalog').mockResolvedValue([SEED_ITEM_WITH_IMAGE])
    render(<CatalogPage />)
    const image = await screen.findByRole('img', { name: SEED_ITEM_WITH_IMAGE.title })
    expect(image).toHaveAttribute('src', SEED_ITEM_WITH_IMAGE.image_url)
  })

  it('renders no photo when the item has no image_url', async () => {
    vi.spyOn(client, 'getCatalog').mockResolvedValue([SEED_ITEM])
    render(<CatalogPage />)
    await waitFor(() => expect(screen.getByText(SEED_ITEM.title)).toBeInTheDocument())
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })
})
```

Also update `frontend/src/api/client.test.ts`: replace its `describe('getCatalog', ...)` block's third test (`'resolves with the catalog items'`) entirely with:

```ts
  it('resolves with the catalog items', async () => {
    const items = [
      {
        id: 1,
        age_range_codes: ['18m'],
        categories: ['toys'],
        title: 'Сортер',
        marketplace_search_url: 'https://example.com',
        image_url: null,
      },
    ]
    mockFetch(200, items)
    await expect(getCatalog({})).resolves.toEqual(items)
  })
```

The other two tests in that `describe` block (requesting with no params, and with `age_range`/`category` params) are unchanged.

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `npm test`
Expected: FAIL — `Failed to resolve import "./categories"`, and `CatalogPage` renders raw `item.age_range_code`/`item.category` (undefined) instead of the new arrays.

- [ ] **Step 3: Implement `categories.ts` and update `client.ts`/`CatalogPage.tsx`**

`frontend/src/catalog/categories.ts`:

```ts
export interface CategoryOption {
  value: string
  label: string
}

export const CATEGORIES: CategoryOption[] = [
  { value: 'clothes', label: 'Одежда' },
  { value: 'toys', label: 'Игрушки' },
  { value: 'books', label: 'Книги' },
  { value: 'sport', label: 'Спорт' },
]
```

In `frontend/src/api/client.ts`, replace the `CatalogItem` interface:

```ts
export interface CatalogItem {
  id: number
  title: string
  marketplace_search_url: string
  image_url: string | null
  age_range_codes: string[]
  categories: string[]
}
```

In `frontend/src/catalog/CatalogPage.tsx`:

1. Add the import: `import { CATEGORIES } from './categories'`.
2. Replace the local `CATEGORY_OPTIONS`/`CATEGORY_LABELS` constants:

```ts
const CATEGORY_OPTIONS = [{ value: '', label: 'Все' }, ...CATEGORIES]
const CATEGORY_LABELS: Record<string, string> = Object.fromEntries(CATEGORIES.map((c) => [c.value, c.label]))
```

3. Replace the card's tags block:

```tsx
<div className="catalog-card-tags">
  {item.age_range_codes.map((code) => (
    <span key={code} className="tag tag-neutral">
      {code}
    </span>
  ))}
  {item.categories.map((cat) => (
    <span key={cat} className="tag tag-outline">
      {CATEGORY_LABELS[cat] ?? cat}
    </span>
  ))}
</div>
```

- [ ] **Step 4: Run the tests to verify they pass**

Run (from `frontend/`): `npm test`
Expected: PASS — all tests.

Run: `npm run build`
Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api/client.ts frontend/src/api/client.test.ts \
        frontend/src/catalog/categories.ts frontend/src/catalog/categories.test.ts \
        frontend/src/catalog/CatalogPage.tsx frontend/src/catalog/CatalogPage.test.tsx
git commit -m "Update catalog card and client for multi-tag items"
```

---

## Task 7: `is_admin` in session; `RequireAdmin` guard; admin nav link

**Files:**
- Modify: `frontend/src/auth/AuthContext.tsx`
- Create: `frontend/src/auth/RequireAdmin.tsx`
- Create: `frontend/src/auth/RequireAdmin.test.tsx`
- Modify: `frontend/src/components/AppNav.tsx`
- Modify: `frontend/src/components/AppNav.test.tsx`

**Interfaces:**
- Consumes: `useAuth` (existing).
- Produces: `Session { email: string; is_admin: boolean }`; `<RequireAdmin>{children}</RequireAdmin>` — redirects to `/` unless `session.is_admin === true`. Task 10 wires `RequireAdmin` around the `/admin` route.

- [ ] **Step 1: Write the failing tests**

`frontend/src/auth/RequireAdmin.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { RequireAdmin } from './RequireAdmin'
import { AuthProvider } from './AuthContext'
import * as client from '../api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AuthProvider>
        <Routes>
          <Route path="/" element={<div>публичная страница</div>} />
          <Route
            path="/admin"
            element={
              <RequireAdmin>
                <div>секретная админка</div>
              </RequireAdmin>
            }
          />
        </Routes>
      </AuthProvider>
    </MemoryRouter>,
  )
}

describe('RequireAdmin', () => {
  it('redirects an anonymous visitor to /', async () => {
    vi.spyOn(client, 'me').mockRejectedValue(new Error('not logged in'))
    renderAt('/admin')
    expect(await screen.findByText('публичная страница')).toBeInTheDocument()
  })

  it('redirects a non-admin session to /', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com', is_admin: false })
    renderAt('/admin')
    expect(await screen.findByText('публичная страница')).toBeInTheDocument()
  })

  it('renders the admin content for an admin session', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'admin@needtobuy.ru', is_admin: true })
    renderAt('/admin')
    expect(await screen.findByText('секретная админка')).toBeInTheDocument()
  })
})
```

Update `frontend/src/components/AppNav.test.tsx` — its two `mockResolvedValue({ email: 'parent@example.com' })` calls need `is_admin: false` added (`Session` will require the field once Step 3 lands), and add one new test. Full replacement of `frontend/src/components/AppNav.test.tsx`:

```tsx
import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AppNav } from './AppNav'
import { AuthProvider } from '../auth/AuthContext'
import * as client from '../api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

describe('AppNav', () => {
  it('shows a login link when anonymous', async () => {
    vi.spyOn(client, 'me').mockRejectedValue(new Error('not logged in'))
    render(
      <MemoryRouter>
        <AuthProvider>
          <AppNav />
        </AuthProvider>
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('Войти')).toBeInTheDocument())
  })

  it('shows a logout button when authenticated', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com', is_admin: false })
    render(
      <MemoryRouter>
        <AuthProvider>
          <AppNav />
        </AuthProvider>
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByRole('button', { name: 'Выйти' })).toBeInTheDocument())
  })

  it('shows neither login nor logout while the session is loading', () => {
    vi.spyOn(client, 'me').mockReturnValue(new Promise(() => {}))
    render(
      <MemoryRouter>
        <AuthProvider>
          <AppNav />
        </AuthProvider>
      </MemoryRouter>,
    )
    expect(screen.queryByText('Войти')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Выйти' })).toBeNull()
  })

  it('does not show an admin link for a non-admin session', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com', is_admin: false })
    render(
      <MemoryRouter>
        <AuthProvider>
          <AppNav />
        </AuthProvider>
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByRole('button', { name: 'Выйти' })).toBeInTheDocument())
    expect(screen.queryByText('Админка')).toBeNull()
  })

  it('shows an admin link for an admin session', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'admin@needtobuy.ru', is_admin: true })
    render(
      <MemoryRouter>
        <AuthProvider>
          <AppNav />
        </AuthProvider>
      </MemoryRouter>,
    )
    await waitFor(() => expect(screen.getByText('Админка')).toBeInTheDocument())
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `npm test`
Expected: FAIL — `Failed to resolve import "./RequireAdmin"`, and the admin-link assertions fail since `AppNav` doesn't render one yet.

- [ ] **Step 3: Implement `AuthContext.tsx`, `RequireAdmin.tsx`, and `AppNav.tsx`**

In `frontend/src/auth/AuthContext.tsx`, replace the `Session` interface and the `fetchMe` handler:

```tsx
export interface Session {
  email: string
  is_admin: boolean
}
```

and inside `AuthProvider`'s `useEffect`:

```tsx
  useEffect(() => {
    fetchMe()
      .then((result) => setSession({ email: result.email, is_admin: result.is_admin }))
      .catch(() => setSession(null))
  }, [])
```

`frontend/src/auth/RequireAdmin.tsx`:

```tsx
import type { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from './useAuth'

export function RequireAdmin({ children }: { children: ReactNode }) {
  const { session } = useAuth()
  if (session === 'loading') return null
  if (session === null || !session.is_admin) return <Navigate to="/" replace />
  return <>{children}</>
}
```

In `frontend/src/components/AppNav.tsx`, add an admin link when `session.is_admin` is true — full replacement:

```tsx
import { Link } from 'react-router-dom'
import { Button } from './Button'
import { logout } from '../api/client'
import { useAuth } from '../auth/useAuth'

export function AppNav() {
  const { session, setSession } = useAuth()

  async function handleLogout() {
    await logout()
    setSession(null)
  }

  return (
    <nav className="nav">
      <span className="nav-brand">Нужняшки</span>
      <span>
        {session === 'loading' ? null : session === null ? (
          <Link to="/login">Войти</Link>
        ) : (
          <>
            {session.is_admin ? <Link to="/admin">Админка</Link> : null}
            <Button variant="ghost" onClick={handleLogout}>
              Выйти
            </Button>
          </>
        )}
      </span>
    </nav>
  )
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run (from `frontend/`): `npm test`
Expected: PASS — all tests.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/auth/AuthContext.tsx frontend/src/auth/RequireAdmin.tsx frontend/src/auth/RequireAdmin.test.tsx \
        frontend/src/components/AppNav.tsx frontend/src/components/AppNav.test.tsx
git commit -m "Add is_admin session field, RequireAdmin guard, admin nav link"
```

---

## Task 8: `CheckboxGroup` component

**Files:**
- Create: `frontend/src/components/CheckboxGroup.tsx`
- Create: `frontend/src/components/CheckboxGroup.test.tsx`
- Modify: `frontend/src/styles/app.css`

**Interfaces:**
- Produces: `<CheckboxGroup legend, options: {value,label}[], values: string[], onChange: (values: string[]) => void>`. Task 9's admin form consumes this.

- [ ] **Step 1: Write the failing tests**

`frontend/src/components/CheckboxGroup.test.tsx`:

```tsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { CheckboxGroup } from './CheckboxGroup'

describe('CheckboxGroup', () => {
  it('renders every option and checks the ones in values', () => {
    render(
      <CheckboxGroup
        legend="Категории"
        options={[
          { value: 'toys', label: 'Игрушки' },
          { value: 'books', label: 'Книги' },
        ]}
        values={['books']}
        onChange={() => {}}
      />,
    )
    expect(screen.getByRole('checkbox', { name: 'Игрушки' })).not.toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Книги' })).toBeChecked()
  })

  it('adds the value when an unchecked option is clicked', async () => {
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(
      <CheckboxGroup
        legend="Категории"
        options={[{ value: 'toys', label: 'Игрушки' }]}
        values={[]}
        onChange={onChange}
      />,
    )
    await user.click(screen.getByRole('checkbox', { name: 'Игрушки' }))
    expect(onChange).toHaveBeenCalledWith(['toys'])
  })

  it('removes the value when a checked option is clicked', async () => {
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(
      <CheckboxGroup
        legend="Категории"
        options={[
          { value: 'toys', label: 'Игрушки' },
          { value: 'books', label: 'Книги' },
        ]}
        values={['toys', 'books']}
        onChange={onChange}
      />,
    )
    await user.click(screen.getByRole('checkbox', { name: 'Игрушки' }))
    expect(onChange).toHaveBeenCalledWith(['books'])
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `npm test`
Expected: FAIL — `Failed to resolve import "./CheckboxGroup"`

- [ ] **Step 3: Implement `CheckboxGroup.tsx`**

```tsx
export interface CheckboxOption {
  value: string
  label: string
}

interface CheckboxGroupProps {
  legend: string
  options: CheckboxOption[]
  values: string[]
  onChange: (values: string[]) => void
}

export function CheckboxGroup({ legend, options, values, onChange }: CheckboxGroupProps) {
  function toggle(value: string) {
    if (values.includes(value)) {
      onChange(values.filter((v) => v !== value))
    } else {
      onChange([...values, value])
    }
  }

  return (
    <fieldset className="checkbox-group">
      <legend>{legend}</legend>
      {options.map((option) => (
        <label key={option.value} className="checkbox-option">
          <input type="checkbox" checked={values.includes(option.value)} onChange={() => toggle(option.value)} />
          {option.label}
        </label>
      ))}
    </fieldset>
  )
}
```

Append to `frontend/src/styles/app.css`:

```css
.checkbox-group {
  border: 1px solid var(--color-divider);
  border-radius: var(--radius-md);
  padding: var(--space-3);
  margin: 0 0 var(--space-3);
}

.checkbox-group legend {
  padding: 0 var(--space-2);
  font-weight: 600;
}

.checkbox-option {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  margin: 0 var(--space-3) var(--space-2) 0;
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run (from `frontend/`): `npm test`
Expected: PASS — all tests.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/CheckboxGroup.tsx frontend/src/components/CheckboxGroup.test.tsx frontend/src/styles/app.css
git commit -m "Add CheckboxGroup component"
```

---

## Task 9: Admin API client + `AdminPage`

**Files:**
- Modify: `frontend/src/api/client.ts`
- Modify: `frontend/src/api/client.test.ts`
- Create: `frontend/src/admin/AdminPage.tsx`
- Create: `frontend/src/admin/AdminPage.test.tsx`

**Interfaces:**
- Consumes: `CheckboxGroup` (Task 8), `CATEGORIES` (Task 6), `AGE_GROUPS`/`AGE_LABELS` (existing, in `catalog/ageGroups.ts`).
- Produces: `AdminCatalogItem`, `CatalogItemInput`, `getAdminCatalog()`, `createCatalogItem(input)`, `updateCatalogItem(id, input)` in `client.ts`; `<AdminPage>`. Task 10 wires `AdminPage` into `/admin`.

- [ ] **Step 1: Write the failing tests**

Append to `frontend/src/api/client.test.ts` (add `getAdminCatalog`, `createCatalogItem`, `updateCatalogItem` to the top-of-file import line, making it `import { requestOtp, verifyOtp, me, getCatalog, getAdminCatalog, createCatalogItem, updateCatalogItem } from './client'`):

```ts
describe('getAdminCatalog', () => {
  it('requests the admin catalog with credentials', async () => {
    const fetchMock = mockFetch(200, [])
    await getAdminCatalog()
    expect(fetchMock).toHaveBeenCalledWith('/api/admin/catalog', expect.objectContaining({ credentials: 'include' }))
  })
})

describe('createCatalogItem', () => {
  it('posts the item body', async () => {
    const fetchMock = mockFetch(201, { id: 1 })
    const input = {
      title: 'Товар',
      marketplace_search_url: 'https://example.com',
      image_url: '',
      age_range_codes: ['18m'],
      categories: ['toys'],
      status: 'published' as const,
    }
    await createCatalogItem(input)
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/catalog',
      expect.objectContaining({ method: 'POST', body: JSON.stringify(input) }),
    )
  })
})

describe('updateCatalogItem', () => {
  it('patches the item by id with a partial body', async () => {
    const fetchMock = mockFetch(200, { id: 1 })
    await updateCatalogItem(1, { status: 'hidden' })
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/admin/catalog/1',
      expect.objectContaining({ method: 'PATCH', body: JSON.stringify({ status: 'hidden' }) }),
    )
  })
})
```

`frontend/src/admin/AdminPage.test.tsx`:

```tsx
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AdminPage } from './AdminPage'
import * as client from '../api/client'

afterEach(() => {
  vi.restoreAllMocks()
})

const EXISTING_ITEM = {
  id: 1,
  title: 'Сортер',
  marketplace_search_url: 'https://example.com',
  image_url: null,
  age_range_codes: ['18m'],
  categories: ['toys'],
  status: 'published' as const,
}

describe('AdminPage', () => {
  it('renders the admin item list', async () => {
    vi.spyOn(client, 'getAdminCatalog').mockResolvedValue([EXISTING_ITEM])
    render(<AdminPage />)
    await waitFor(() => expect(screen.getByText('Сортер')).toBeInTheDocument())
  })

  it('creates a new item from the form', async () => {
    vi.spyOn(client, 'getAdminCatalog').mockResolvedValue([])
    const createSpy = vi.spyOn(client, 'createCatalogItem').mockResolvedValue(EXISTING_ITEM)
    const user = userEvent.setup()
    render(<AdminPage />)

    await user.click(await screen.findByRole('button', { name: 'Добавить товар' }))
    await user.type(screen.getByLabelText('Заголовок'), 'Новый товар')
    await user.type(screen.getByLabelText('Ссылка на Ozon'), 'https://ozon.ru/search/?text=x')
    await user.click(screen.getByRole('checkbox', { name: '1 год 6 мес.' }))
    await user.click(screen.getByRole('checkbox', { name: 'Игрушки' }))
    await user.click(screen.getByRole('button', { name: 'Сохранить' }))

    await waitFor(() =>
      expect(createSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          title: 'Новый товар',
          marketplace_search_url: 'https://ozon.ru/search/?text=x',
          age_range_codes: ['18m'],
          categories: ['toys'],
          status: 'published',
        }),
      ),
    )
  })

  it('pre-fills the form when editing an existing item', async () => {
    vi.spyOn(client, 'getAdminCatalog').mockResolvedValue([EXISTING_ITEM])
    const user = userEvent.setup()
    render(<AdminPage />)

    await user.click(await screen.findByRole('button', { name: 'Редактировать' }))

    expect(screen.getByLabelText('Заголовок')).toHaveValue('Сортер')
    expect(screen.getByRole('checkbox', { name: '1 год 6 мес.' })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Игрушки' })).toBeChecked()
  })

  it('submits an update with the toggled status', async () => {
    vi.spyOn(client, 'getAdminCatalog').mockResolvedValue([EXISTING_ITEM])
    const updateSpy = vi.spyOn(client, 'updateCatalogItem').mockResolvedValue({ ...EXISTING_ITEM, status: 'hidden' })
    const user = userEvent.setup()
    render(<AdminPage />)

    await user.click(await screen.findByRole('button', { name: 'Редактировать' }))
    await user.click(screen.getByRole('checkbox', { name: 'Опубликован' }))
    await user.click(screen.getByRole('button', { name: 'Сохранить' }))

    await waitFor(() =>
      expect(updateSpy).toHaveBeenCalledWith(1, expect.objectContaining({ status: 'hidden' })),
    )
  })
})
```

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `npm test`
Expected: FAIL — `undefined: getAdminCatalog`/`createCatalogItem`/`updateCatalogItem` and `Failed to resolve import "./AdminPage"`.

- [ ] **Step 3: Implement the client additions and `AdminPage`**

Append to `frontend/src/api/client.ts`:

```ts
export interface AdminCatalogItem extends CatalogItem {
  status: 'published' | 'hidden'
}

export function getAdminCatalog(): Promise<AdminCatalogItem[]> {
  return request('/api/admin/catalog')
}

export interface CatalogItemInput {
  title: string
  marketplace_search_url: string
  image_url: string
  age_range_codes: string[]
  categories: string[]
  status: 'published' | 'hidden'
}

export function createCatalogItem(input: CatalogItemInput): Promise<AdminCatalogItem> {
  return request('/api/admin/catalog', {
    method: 'POST',
    body: JSON.stringify(input),
  })
}

export function updateCatalogItem(id: number, input: Partial<CatalogItemInput>): Promise<AdminCatalogItem> {
  return request(`/api/admin/catalog/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}
```

`frontend/src/admin/AdminPage.tsx`:

```tsx
import { useEffect, useState, type FormEvent } from 'react'
import { CheckboxGroup } from '../components/CheckboxGroup'
import {
  getAdminCatalog,
  createCatalogItem,
  updateCatalogItem,
  ApiError,
  type AdminCatalogItem,
  type CatalogItemInput,
} from '../api/client'
import { AGE_GROUPS, AGE_LABELS } from '../catalog/ageGroups'
import { CATEGORIES } from '../catalog/categories'

const GENERIC_ERROR = 'Что-то пошло не так, попробуйте ещё раз'

const BLANK_FORM: CatalogItemInput = {
  title: '',
  marketplace_search_url: '',
  image_url: '',
  age_range_codes: [],
  categories: [],
  status: 'published',
}

export function AdminPage() {
  const [items, setItems] = useState<AdminCatalogItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editingID, setEditingID] = useState<number | 'new' | null>(null)
  const [form, setForm] = useState<CatalogItemInput>(BLANK_FORM)
  const [formError, setFormError] = useState<string | null>(null)

  function loadItems() {
    setLoading(true)
    setError(null)
    getAdminCatalog()
      .then(setItems)
      .catch((err) => setError(err instanceof ApiError ? err.message : GENERIC_ERROR))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    loadItems()
  }, [])

  function startCreate() {
    setForm(BLANK_FORM)
    setFormError(null)
    setEditingID('new')
  }

  function startEdit(item: AdminCatalogItem) {
    setForm({
      title: item.title,
      marketplace_search_url: item.marketplace_search_url,
      image_url: item.image_url ?? '',
      age_range_codes: item.age_range_codes,
      categories: item.categories,
      status: item.status,
    })
    setFormError(null)
    setEditingID(item.id)
  }

  function cancelEdit() {
    setEditingID(null)
  }

  async function submitForm(event: FormEvent) {
    event.preventDefault()
    setFormError(null)
    try {
      if (editingID === 'new') {
        await createCatalogItem(form)
      } else if (editingID !== null) {
        await updateCatalogItem(editingID, form)
      }
      setEditingID(null)
      loadItems()
    } catch (err) {
      setFormError(err instanceof ApiError ? err.message : GENERIC_ERROR)
    }
  }

  if (editingID !== null) {
    return (
      <div className="catalog-content">
        <h1>Админка каталога</h1>
        <form onSubmit={submitForm} className="card">
          <div className="field">
            <label htmlFor="admin-title">Заголовок</label>
            <input
              id="admin-title"
              className="input"
              value={form.title}
              onChange={(e) => setForm({ ...form, title: e.target.value })}
            />
          </div>
          <div className="field">
            <label htmlFor="admin-url">Ссылка на Ozon</label>
            <input
              id="admin-url"
              className="input"
              value={form.marketplace_search_url}
              onChange={(e) => setForm({ ...form, marketplace_search_url: e.target.value })}
            />
          </div>
          <div className="field">
            <label htmlFor="admin-image">Ссылка на фото (необязательно)</label>
            <input
              id="admin-image"
              className="input"
              value={form.image_url}
              onChange={(e) => setForm({ ...form, image_url: e.target.value })}
            />
          </div>
          {AGE_GROUPS.map((group) => (
            <CheckboxGroup
              key={group.label}
              legend={group.label}
              options={group.codes.map((code) => ({ value: code, label: AGE_LABELS[code] ?? code }))}
              values={form.age_range_codes}
              onChange={(values) => setForm({ ...form, age_range_codes: values })}
            />
          ))}
          <CheckboxGroup
            legend="Категории"
            options={CATEGORIES}
            values={form.categories}
            onChange={(values) => setForm({ ...form, categories: values })}
          />
          <label className="checkbox-option">
            <input
              type="checkbox"
              checked={form.status === 'published'}
              onChange={(e) => setForm({ ...form, status: e.target.checked ? 'published' : 'hidden' })}
            />
            Опубликован
          </label>
          {formError ? <p className="error-text">{formError}</p> : null}
          <div>
            <button type="submit" className="btn btn-primary">
              Сохранить
            </button>
            <button type="button" className="btn btn-ghost" onClick={cancelEdit}>
              Отмена
            </button>
          </div>
        </form>
      </div>
    )
  }

  return (
    <div className="catalog-content">
      <h1>Админка каталога</h1>
      {error ? <p className="error-text">{error}</p> : null}
      <button className="btn btn-primary" onClick={startCreate}>
        Добавить товар
      </button>
      {!loading && items.length === 0 ? <p className="card-body">Пока нет товаров.</p> : null}
      <ul className="admin-list">
        {items.map((item) => (
          <li key={item.id} className="admin-list-item card">
            <div>
              <strong>{item.title}</strong>{' '}
              <span className="tag tag-neutral">{item.status === 'published' ? 'опубликован' : 'скрыт'}</span>
              <div className="catalog-card-tags">
                {item.age_range_codes.map((code) => (
                  <span key={code} className="tag tag-neutral">
                    {code}
                  </span>
                ))}
                {item.categories.map((cat) => (
                  <span key={cat} className="tag tag-outline">
                    {CATEGORIES.find((c) => c.value === cat)?.label ?? cat}
                  </span>
                ))}
              </div>
            </div>
            <button className="btn btn-secondary" onClick={() => startEdit(item)}>
              Редактировать
            </button>
          </li>
        ))}
      </ul>
    </div>
  )
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run (from `frontend/`): `npm test`
Expected: PASS — all tests.

Run: `npm run build`
Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/api/client.ts frontend/src/api/client.test.ts frontend/src/admin
git commit -m "Add admin API client and AdminPage"
```

---

## Task 10: Wire `/admin` into `App`

**Files:**
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/App.test.tsx`

**Interfaces:**
- Consumes: `RequireAdmin` (Task 7), `AdminPage` (Task 9).
- Produces: the final `App` — nothing later in this plan depends on it.

- [ ] **Step 1: Write the failing test**

Append to `frontend/src/App.test.tsx` (inside the existing `describe('App routing', ...)` block, after the last test):

```tsx
  it('redirects a non-admin session away from /admin to the catalog', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'parent@example.com', is_admin: false })
    vi.spyOn(client, 'getCatalog').mockResolvedValue([])

    render(
      <MemoryRouter initialEntries={['/admin']}>
        <App />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByText('Идеи по возрасту')).toBeInTheDocument())
  })

  it('shows the admin page for an admin session', async () => {
    vi.spyOn(client, 'me').mockResolvedValue({ email: 'admin@needtobuy.ru', is_admin: true })
    vi.spyOn(client, 'getAdminCatalog').mockResolvedValue([])

    render(
      <MemoryRouter initialEntries={['/admin']}>
        <App />
      </MemoryRouter>,
    )

    await waitFor(() => expect(screen.getByText('Админка каталога')).toBeInTheDocument())
  })
```

Also add `is_admin: false` to the existing test `'shows a logout button in the nav when authenticated'`'s `mockResolvedValue({ email: 'parent@example.com' })` call (making it `{ email: 'parent@example.com', is_admin: false }`), and the same for `'logs out and returns to the catalog with a login link'`'s `mockResolvedValue({ email: 'parent@example.com' })` call. Both are pre-existing calls in the current file — every other line in `App.test.tsx` stays as-is.

- [ ] **Step 2: Run to verify it fails**

Run (from `frontend/`): `npm test`
Expected: FAIL — `/admin` isn't a registered route yet, so both new tests time out waiting for content that never renders.

- [ ] **Step 3: Implement `App.tsx`**

Replace `frontend/src/App.tsx` entirely with:

```tsx
import { Navigate, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth/AuthContext'
import { useAuth } from './auth/useAuth'
import { LoginPage } from './auth/LoginPage'
import { RequireAdmin } from './auth/RequireAdmin'
import { AppNav } from './components/AppNav'
import { CatalogPage } from './catalog/CatalogPage'
import { AdminPage } from './admin/AdminPage'

function LoginRoute() {
  const { session } = useAuth()
  if (session === 'loading') return null
  if (session !== null) return <Navigate to="/" replace />
  return <LoginPage />
}

function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<LoginRoute />} />
      <Route
        path="/"
        element={
          <>
            <AppNav />
            <CatalogPage />
          </>
        }
      />
      <Route
        path="/admin"
        element={
          <RequireAdmin>
            <AppNav />
            <AdminPage />
          </RequireAdmin>
        }
      />
    </Routes>
  )
}

export default function App() {
  return (
    <AuthProvider>
      <AppRoutes />
    </AuthProvider>
  )
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run (from `frontend/`): `npm test`
Expected: PASS — all tests.

Run: `npm run build`
Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/App.tsx frontend/src/App.test.tsx
git commit -m "Wire /admin route into App"
```

- [ ] **Step 6: Manual end-to-end smoke test against the real backend**

From the repo root: `docker compose up -d --build`. Open `http://localhost:5173`. Log in as `babaliants@gmail.com` via mailcatcher (`http://localhost:1080`) — expected: nav shows «Админка» after login. Click it — expected: list of the 18 seeded items (now each showing one age tag + one category tag, migrated 1:1 from the old schema). Click «Добавить товар», fill the form (check a couple of age-range boxes across different groups and a couple of category boxes), submit — expected: back on the list, new item visible with all the tags you checked. Open the public catalog at `/` — expected: the new item appears there too (if published) with all its tags rendered as separate pills. Log out, log back in as a different (non-admin) email — expected: no «Админка» link in the nav, and navigating to `/admin` directly redirects to `/`. Stop the stack afterward with `docker compose down`.

---

## Self-review notes

- **Spec coverage:** multi-tag join tables + status column, existing data migrated → Task 1; public list filters by any-tag-match + published-only, no status leak → Tasks 2-3; admin list/create/update, no-delete, tag-replace-not-merge PATCH semantics → Tasks 2-3; `RequireAdmin` gating + `is_admin` in `/me` + `config.AdminEmail` → Task 4; router wiring + 401/403 e2e proof → Task 5; public card renders multiple tags → Task 6; frontend admin gating (`RequireAdmin`, nav link) → Task 7; `CheckboxGroup` → Task 8; admin form (title/url/image/age-groups/categories/publish-toggle) → Task 9; `/admin` route → Task 10.
- **Signature ripple discipline:** both `auth.NewHandler` (4th param) and the already-4-arg `httpapi.NewRouter` (unchanged signature, but every call site still needed the `NewHandler` argument added) are handled with full-file replacements for every touched call site in Tasks 4-5, not prose edits — mirroring the catalog plan's router-signature-change task.
- **Cross-task type consistency:** `item{ID, Title, MarketplaceSearchURL, ImageURL, Status, AgeRangeCodes, Categories}` (Task 2) is consumed unchanged by `toResponse`/`toAdminResponse` (Task 3); `CatalogItemInput`/`AdminCatalogItem` (Task 9) match the JSON field names `adminCreateBody`/`adminUpdateBody` (Task 3) expect (`age_range_codes`, `categories`, `image_url`, `status`); `RequireAdmin` (Task 7) reads `session.is_admin`, matching the `Session` interface Task 7 itself defines and the `is_admin` boolean Task 4's `Me` handler emits.
- **Placeholder scan:** none found — every step carries complete code, no "TBD"/"add validation"/"similar to Task N" language.
- **Out of scope, confirmed unblocked for later slices:** AI generation (`catalog.Suggester`, `source`/`approved_at` columns — additive migration later, this plan's `status` column is untouched by that); full delete (would be a new `DELETE` route + repository function, no schema conflict); photo upload (would replace the plain `image_url` text field's input with a file picker + a new storage-backed endpoint, no schema conflict since `image_url` stays a URL either way).
