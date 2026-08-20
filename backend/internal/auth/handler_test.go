package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"needtobuy/internal/dbtest"
)

// capturingMailer records the last code it was asked to send, so tests can
// read back the OTP that would have gone out by email.
type capturingMailer struct {
	lastCode string
}

func (m *capturingMailer) SendOTP(_ context.Context, _ string, code string) error {
	m.lastCode = code
	return nil
}

func newTestHandler(t *testing.T) (*Handler, *capturingMailer) {
	t.Helper()
	mailer := &capturingMailer{}
	return NewHandler(dbtest.Tx(t), mailer, "pepper"), mailer
}

func TestRequestOTP_ValidEmail_SendsCode(t *testing.T) {
	h, mailer := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"parent@example.com"}`))
	rec := httptest.NewRecorder()

	h.RequestOTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(mailer.lastCode) != 6 {
		t.Fatalf("mailer.lastCode = %q, want 6 digits", mailer.lastCode)
	}
}

func TestRequestOTP_MissingEmail_BadRequest(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()

	h.RequestOTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestRequestOTP_EmailWithCRLF_BadRequest(t *testing.T) {
	h, _ := newTestHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"a@example.com\r\nBcc: evil@example.com"}`))
	rec := httptest.NewRecorder()

	h.RequestOTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestVerifyOTP_CorrectCode_SetsSessionCookie(t *testing.T) {
	h, mailer := newTestHandler(t)

	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"parent@example.com"}`))
	h.RequestOTP(httptest.NewRecorder(), reqReq)

	body := `{"email":"parent@example.com","code":"` + mailer.lastCode + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.VerifyOTP(rec, verifyReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != sessionCookie || cookies[0].Value == "" {
		t.Fatalf("cookies = %+v, want one non-empty %q cookie", cookies, sessionCookie)
	}
}

func TestVerifyOTP_WrongCode_BadRequest(t *testing.T) {
	h, mailer := newTestHandler(t)

	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"parent@example.com"}`))
	h.RequestOTP(httptest.NewRecorder(), reqReq)

	wrong := "000000"
	if wrong == mailer.lastCode {
		wrong = "000001"
	}
	body := `{"email":"parent@example.com","code":"` + wrong + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.VerifyOTP(rec, verifyReq)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestMiddleware_ProtectedRoute_RequiresSession(t *testing.T) {
	h, mailer := newTestHandler(t)

	protected := h.Middleware(RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := UserID(r.Context())
		json.NewEncoder(w).Encode(map[string]int64{"user_id": id})
	})))

	noCookie := httptest.NewRequest(http.MethodGet, "/protected", nil)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, noCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie: status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"parent@example.com"}`))
	h.RequestOTP(httptest.NewRecorder(), reqReq)
	verifyBody := `{"email":"parent@example.com","code":"` + mailer.lastCode + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	h.VerifyOTP(verifyRec, verifyReq)
	sessionCookieValue := verifyRec.Result().Cookies()[0]

	withCookie := httptest.NewRequest(http.MethodGet, "/protected", nil)
	withCookie.AddCookie(sessionCookieValue)
	rec = httptest.NewRecorder()
	protected.ServeHTTP(rec, withCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("with cookie: status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestLogout_ClearsCookieAndInvalidatesSession(t *testing.T) {
	h, mailer := newTestHandler(t)

	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"parent@example.com"}`))
	h.RequestOTP(httptest.NewRecorder(), reqReq)
	verifyBody := `{"email":"parent@example.com","code":"` + mailer.lastCode + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	h.VerifyOTP(verifyRec, verifyReq)
	sessionCookieValue := verifyRec.Result().Cookies()[0]

	logoutReq := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logoutReq.AddCookie(sessionCookieValue)
	logoutRec := httptest.NewRecorder()
	h.Logout(logoutRec, logoutReq)

	if logoutRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", logoutRec.Code, http.StatusOK, logoutRec.Body.String())
	}
	cleared := logoutRec.Result().Cookies()[0]
	if cleared.MaxAge >= 0 {
		t.Fatalf("logout cookie MaxAge = %d, want negative (expired)", cleared.MaxAge)
	}

	protected := h.Middleware(RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	afterLogout := httptest.NewRequest(http.MethodGet, "/protected", nil)
	afterLogout.AddCookie(sessionCookieValue)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, afterLogout)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("after logout: status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMe_NoCookie_Unauthorized(t *testing.T) {
	h, _ := newTestHandler(t)
	protected := h.Middleware(RequireAuth(http.HandlerFunc(h.Me)))

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	rec := httptest.NewRecorder()
	protected.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestMe_ValidSession_ReturnsEmail(t *testing.T) {
	h, mailer := newTestHandler(t)
	protected := h.Middleware(RequireAuth(http.HandlerFunc(h.Me)))

	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"parent@example.com"}`))
	h.RequestOTP(httptest.NewRecorder(), reqReq)
	verifyBody := `{"email":"parent@example.com","code":"` + mailer.lastCode + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	h.VerifyOTP(verifyRec, verifyReq)
	sessionCookieValue := verifyRec.Result().Cookies()[0]

	meReq := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	meReq.AddCookie(sessionCookieValue)
	meRec := httptest.NewRecorder()
	protected.ServeHTTP(meRec, meReq)

	if meRec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", meRec.Code, http.StatusOK, meRec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(meRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal body: %v", err)
	}
	if body["email"] != "parent@example.com" {
		t.Fatalf(`email = %q, want "parent@example.com"`, body["email"])
	}
}
