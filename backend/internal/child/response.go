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
