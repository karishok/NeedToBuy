package catalog

// itemResponse is the JSON shape returned by GET /api/catalog.
type itemResponse struct {
	ID                   int64  `json:"id"`
	AgeRangeCode         string `json:"age_range_code"`
	Category             string `json:"category"`
	Title                string `json:"title"`
	MarketplaceSearchURL string `json:"marketplace_search_url"`
}

func toResponse(r row) itemResponse {
	return itemResponse{
		ID:                   r.ID,
		AgeRangeCode:         r.AgeRangeCode,
		Category:             r.Category,
		Title:                r.Title,
		MarketplaceSearchURL: r.MarketplaceSearchURL,
	}
}
