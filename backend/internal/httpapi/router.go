package httpapi

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"

	"needtobuy/internal/apierr"
	"needtobuy/internal/auth"
)

// NewRouter builds the top-level chi router. database is used by the
// health check to verify connectivity; authHandler registers the OTP and
// logout endpoints and its Middleware runs on every request so downstream
// handlers can read the authenticated parent via auth.UserID.
func NewRouter(database *sqlx.DB, authHandler *auth.Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(authHandler.Middleware)

	r.Get("/healthz", healthHandler(database))

	r.Post("/api/auth/otp/request", authHandler.RequestOTP)
	r.Post("/api/auth/otp/verify", authHandler.VerifyOTP)
	r.Post("/api/auth/logout", authHandler.Logout)

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
