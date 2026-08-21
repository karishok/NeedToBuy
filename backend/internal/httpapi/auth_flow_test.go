package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"needtobuy/internal/auth"
	"needtobuy/internal/child"
	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
	"needtobuy/internal/httpapi"
)

// capturingMailer records the last code it was asked to send.
//
// This test writes one real, non-transactional user/session row (unlike
// the rest of the suite, which uses dbtest.Tx) because httpapi.NewRouter
// needs a live *sqlx.DB for its health check, not a transaction. The email
// is randomized per run so repeat runs don't collide; the local dev
// database is disposable (`docker compose down -v` resets it).
type capturingMailer struct {
	lastCode string
}

func (m *capturingMailer) SendOTP(_ context.Context, _ string, code string) error {
	m.lastCode = code
	return nil
}

func TestAuthFlow_RequestVerifyLogout_EndToEnd(t *testing.T) {
	dsn := dbtest.DSN(t)
	if err := db.Migrate(dsn); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	mailer := &capturingMailer{}
	authHandler := auth.NewHandler(conn, mailer, "pepper")
	childHandler := child.NewHandler(conn)
	router := httpapi.NewRouter(conn, authHandler, childHandler)
	email := fmt.Sprintf("e2e-%d@example.com", time.Now().UnixNano())

	reqBody := fmt.Sprintf(`{"email":%q}`, email)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("request: status = %d, body = %s", rec.Code, rec.Body.String())
	}

	verifyBody := fmt.Sprintf(`{"email":%q,"code":%q}`, email, mailer.lastCode)
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	router.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("verify: status = %d, body = %s", verifyRec.Code, verifyRec.Body.String())
	}
	cookies := verifyRec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("verify: no session cookie set")
	}
	sessionCookie := cookies[0]

	// No cookie at all: /api/auth/me must reject.
	noCookieReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	noCookieRec := httptest.NewRecorder()
	router.ServeHTTP(noCookieRec, noCookieReq)
	if noCookieRec.Code != http.StatusUnauthorized {
		t.Fatalf("me without cookie: status = %d, want %d", noCookieRec.Code, http.StatusUnauthorized)
	}

	// With the real session cookie: /api/auth/me must report the email.
	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(sessionCookie)
	meRec := httptest.NewRecorder()
	router.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me with cookie: status = %d, body = %s", meRec.Code, meRec.Body.String())
	}
	var me map[string]string
	if err := json.Unmarshal(meRec.Body.Bytes(), &me); err != nil {
		t.Fatalf("unmarshal me body: %v", err)
	}
	if me["email"] != email {
		t.Fatalf("me email = %q, want %q", me["email"], email)
	}

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthReq.AddCookie(sessionCookie)
	healthRec := httptest.NewRecorder()
	router.ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("healthz with session cookie: status = %d, body = %s", healthRec.Code, healthRec.Body.String())
	}
	var health map[string]string
	if err := json.Unmarshal(healthRec.Body.Bytes(), &health); err != nil {
		t.Fatalf("unmarshal healthz body: %v", err)
	}
	if health["status"] != "ok" {
		t.Fatalf(`healthz status = %q, want "ok"`, health["status"])
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutRec := httptest.NewRecorder()
	router.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK {
		t.Fatalf("logout: status = %d, body = %s", logoutRec.Code, logoutRec.Body.String())
	}

	// Same session cookie after logout: the session must be revoked.
	postLogoutReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	postLogoutReq.AddCookie(sessionCookie)
	postLogoutRec := httptest.NewRecorder()
	router.ServeHTTP(postLogoutRec, postLogoutReq)
	if postLogoutRec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout: status = %d, want %d", postLogoutRec.Code, http.StatusUnauthorized)
	}
}
