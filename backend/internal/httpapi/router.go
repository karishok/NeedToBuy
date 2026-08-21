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
// browsing endpoint — no authentication required, same as /healthz.
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
