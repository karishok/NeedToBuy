package catalog

// itemResponse is the JSON shape returned by GET /api/catalog. ImageURL
// is always present in the JSON (never omitted) so the frontend can
// distinguish "no photo yet" (null) from a missing field; it is nil
// until a catalog item has a curated photo.
type itemResponse struct {
	ID                   int64   `json:"id"`
	AgeRangeCode         string  `json:"age_range_code"`
	Category             string  `json:"category"`
	Title                string  `json:"title"`
	MarketplaceSearchURL string  `json:"marketplace_search_url"`
	ImageURL             *string `json:"image_url"`
}

func toResponse(r row) itemResponse {
	resp := itemResponse{
		ID:                   r.ID,
		AgeRangeCode:         r.AgeRangeCode,
		Category:             r.Category,
		Title:                r.Title,
		MarketplaceSearchURL: r.MarketplaceSearchURL,
	}
	if r.ImageURL.Valid {
		resp.ImageURL = &r.ImageURL.String
	}
	return resp
}
