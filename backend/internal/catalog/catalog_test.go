package catalog

import (
	"context"
	"testing"

	"needtobuy/internal/dbtest"
)

func TestListCatalogItems_NoFilters_ReturnsSeedData(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	rows, err := listCatalogItems(ctx, tx, "", "")
	if err != nil {
		t.Fatalf("listCatalogItems() error = %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("listCatalogItems() returned no rows, want the seeded catalog data")
	}
}

func TestListCatalogItems_FiltersByAgeRange(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	rows, err := listCatalogItems(ctx, tx, "18m", "")
	if err != nil {
		t.Fatalf("listCatalogItems() error = %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one seeded item at 18m")
	}
	for _, r := range rows {
		if r.AgeRangeCode != "18m" {
			t.Fatalf("row age_range_code = %q, want 18m", r.AgeRangeCode)
		}
	}
}

func TestListCatalogItems_FiltersByCategory(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	rows, err := listCatalogItems(ctx, tx, "", "sport")
	if err != nil {
		t.Fatalf("listCatalogItems() error = %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("expected at least one seeded sport item")
	}
	for _, r := range rows {
		if r.Category != "sport" {
			t.Fatalf("row category = %q, want sport", r.Category)
		}
	}
}

func TestListCatalogItems_FiltersByBoth_NoMatch_ReturnsEmpty(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	// The seed data (migration 000005) has no clothes item at 12y+ — only
	// a sport item there. If the seed data ever changes, update this test
	// to name a combination that's still guaranteed empty.
	rows, err := listCatalogItems(ctx, tx, "12y+", "clothes")
	if err != nil {
		t.Fatalf("listCatalogItems() error = %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("listCatalogItems() = %+v, want empty for this combination", rows)
	}
}
