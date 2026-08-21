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
