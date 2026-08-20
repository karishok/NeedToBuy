package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"
)

// NewRouter builds the top-level chi router. database is used by the
// health check to verify connectivity; domain packages register their
// own routes on the returned handler in later plans.
func NewRouter(database *sqlx.DB) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/healthz", healthHandler(database))

	return r
}

func healthHandler(database *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := database.PingContext(r.Context()); err != nil {
			WriteError(w, Internal("database unavailable"))
			return
		}
		WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
