package catalog

import (
	"context"
	"fmt"

	"needtobuy/internal/auth"
)

// row mirrors one row of the catalog_items table as scanned from
// Postgres.
type row struct {
	ID                   int64  `db:"id"`
	AgeRangeCode         string `db:"age_range_code"`
	Category             string `db:"category"`
	Title                string `db:"title"`
	MarketplaceSearchURL string `db:"marketplace_search_url"`
}

// listCatalogItems returns catalog items matching the given filters.
// An empty ageRange or category means "no filter on that field."
func listCatalogItems(ctx context.Context, db auth.Querier, ageRange, category string) ([]row, error) {
	query := `SELECT id, age_range_code, category, title, marketplace_search_url FROM catalog_items WHERE 1=1`
	args := []any{}
	argN := 1
	if ageRange != "" {
		query += fmt.Sprintf(" AND age_range_code = $%d", argN)
		args = append(args, ageRange)
		argN++
	}
	if category != "" {
		query += fmt.Sprintf(" AND category = $%d", argN)
		args = append(args, category)
	}
	query += " ORDER BY id"

	var rows []row
	if err := db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("catalog: list: %w", err)
	}
	return rows, nil
}
