package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"needtobuy/internal/apierr"
)

// Handler wires the OTP and session HTTP endpoints to a database and
// mailer.
type Handler struct {
	db     Querier
	mailer Mailer
	pepper string
}

// NewHandler builds a Handler ready to register on a router.
func NewHandler(database Querier, mailer Mailer, pepper string) *Handler {
	return &Handler{db: database, mailer: mailer, pepper: pepper}
}

type requestOTPBody struct {
	Email string `json:"email"`
}

// RequestOTP handles POST /api/auth/otp/request.
func (h *Handler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	var body requestOTPBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierr.WriteError(w, apierr.BadRequest("invalid request body"))
		return
	}
	email := normalizeEmail(body.Email)
	if email == "" {
		apierr.WriteError(w, apierr.BadRequest("email is required"))
		return
	}

	code, err := createOTP(r.Context(), h.db, h.pepper, email)
	if errors.Is(err, errTooSoon) {
		apierr.WriteError(w, tooManyRequests("code already sent, try again shortly"))
		return
	}
	if err != nil {
		log.Printf("auth: create otp for %s: %v", email, err)
		apierr.WriteError(w, apierr.Internal("could not create code"))
		return
	}

	if err := h.mailer.SendOTP(r.Context(), email, code); err != nil {
		log.Printf("auth: send otp mail to %s: %v", email, err)
		if delErr := deleteOTP(r.Context(), h.db, email); delErr != nil {
			log.Printf("auth: cleanup otp after failed send for %s: %v", email, delErr)
		}
		apierr.WriteError(w, apierr.Internal("could not send code"))
		return
	}

	apierr.WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

type verifyOTPBody struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

// VerifyOTP handles POST /api/auth/otp/verify.
//
// The code-consume, user-upsert, and session-create steps below are three
// separate statements, not one transaction — Querier only exposes
// GetContext/ExecContext, not a way to begin one (see db.go). If
// upsertUser or createSession fails after verifyOTP has already consumed
// the code, the code is unrecoverably burned and the caller must request
// a new one. Accepted for this MVP slice; revisit if this becomes a real
// support burden.
func (h *Handler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var body verifyOTPBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierr.WriteError(w, apierr.BadRequest("invalid request body"))
		return
	}
	email := normalizeEmail(body.Email)
	if email == "" || body.Code == "" {
		apierr.WriteError(w, apierr.BadRequest("email and code are required"))
		return
	}

	err := verifyOTP(r.Context(), h.db, h.pepper, email, body.Code)
	if errors.Is(err, errInvalidCode) {
		apierr.WriteError(w, apierr.BadRequest("invalid or expired code"))
		return
	}
	if err != nil {
		log.Printf("auth: verify otp for %s: %v", email, err)
		apierr.WriteError(w, apierr.Internal("could not verify code"))
		return
	}

	userID, err := upsertUser(r.Context(), h.db, email)
	if err != nil {
		log.Printf("auth: upsert user for %s: %v", email, err)
		apierr.WriteError(w, apierr.Internal("could not create account"))
		return
	}

	sessionID, err := createSession(r.Context(), h.db, userID)
	if err != nil {
		log.Printf("auth: create session for user %d: %v", userID, err)
		apierr.WriteError(w, apierr.Internal("could not create session"))
		return
	}

	setSessionCookie(w, sessionID)
	apierr.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Logout handles POST /api/auth/logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_ = deleteSession(r.Context(), h.db, cookie.Value)
	}
	clearSessionCookie(w)
	apierr.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Me handles GET /api/auth/me. It must be registered behind RequireAuth —
// it reports 401 defensively if that invariant is ever violated.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserID(r.Context())
	if !ok {
		apierr.WriteError(w, unauthorized("login required"))
		return
	}
	email, err := emailByUserID(r.Context(), h.db, userID)
	if err != nil {
		log.Printf("auth: lookup email for user %d: %v", userID, err)
		apierr.WriteError(w, apierr.Internal("could not load account"))
		return
	}
	apierr.WriteJSON(w, http.StatusOK, map[string]string{"email": email})
}

// Middleware loads the session cookie, if any, and stores the parent's user
// id in the request context for downstream handlers to read via UserID. It
// never rejects a request itself — wrap routes that require login in
// RequireAuth as well.
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		userID, ok, err := lookupSession(r.Context(), h.db, cookie.Value)
		if err != nil || !ok {
			next.ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), userIDContextKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAuth wraps next so it responds 401 unless Middleware has already
// attached a user id to the request context. Domain packages (child,
// wishlist, catalog) wrap their protected routes in this.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := UserID(r.Context()); !ok {
			apierr.WriteError(w, unauthorized("login required"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func tooManyRequests(message string) *apierr.Error {
	return &apierr.Error{Code: "too_many_requests", Message: message, HTTPStatus: http.StatusTooManyRequests}
}

func unauthorized(message string) *apierr.Error {
	return &apierr.Error{Code: "unauthorized", Message: message, HTTPStatus: http.StatusUnauthorized}
}

// normalizeEmail trims and lowercases an email string. It returns an empty
// string if the email contains CRLF characters that could be used for SMTP
// header injection.
func normalizeEmail(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if strings.Contains(normalized, "\r") || strings.Contains(normalized, "\n") {
		return ""
	}
	return normalized
}
