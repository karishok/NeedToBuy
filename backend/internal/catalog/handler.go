package catalog

import (
	"log"
	"net/http"

	"needtobuy/internal/agerange"
	"needtobuy/internal/apierr"
	"needtobuy/internal/auth"
)

// Handler wires the catalog HTTP endpoint to a database.
type Handler struct {
	db auth.Querier
}

// NewHandler builds a Handler ready to register on a router.
func NewHandler(database auth.Querier) *Handler {
	return &Handler{db: database}
}

// List handles GET /api/catalog. No authentication is required — the
// catalog is a public reference, not tied to any parent's account.
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

	rows, err := listCatalogItems(r.Context(), h.db, ageRange, category)
	if err != nil {
		log.Printf("catalog: list age_range=%q category=%q: %v", ageRange, category, err)
		apierr.WriteError(w, apierr.Internal("could not load catalog"))
		return
	}
	responses := make([]itemResponse, len(rows))
	for i, item := range rows {
		responses[i] = toResponse(item)
	}
	apierr.WriteJSON(w, http.StatusOK, responses)
}
