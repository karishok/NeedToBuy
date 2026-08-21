package child

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"needtobuy/internal/auth"
	"needtobuy/internal/dbtest"
)

// capturingMailer records the last code it was asked to send, so tests
// can read back the OTP that would have gone out by email.
type capturingMailer struct {
	lastCode string
}

func (m *capturingMailer) SendOTP(_ context.Context, _ string, code string) error {
	m.lastCode = code
	return nil
}

// testEnv bundles a child.Handler under test with a real auth.Handler
// sharing the same transaction, so tests can mint real session cookies
// and route requests through auth.Handler.Middleware exactly as
// production's router does — child handlers read the authenticated
// parent via auth.UserID(r.Context()), which only Middleware populates.
type testEnv struct {
	authHandler  *auth.Handler
	childHandler *Handler
	mailer       *capturingMailer
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tx := dbtest.Tx(t)
	mailer := &capturingMailer{}
	return &testEnv{
		authHandler:  auth.NewHandler(tx, mailer, "pepper"),
		childHandler: NewHandler(tx),
		mailer:       mailer,
	}
}

// login creates (or logs into) a parent by email and returns a session
// cookie for them.
func (e *testEnv) login(t *testing.T, email string) *http.Cookie {
	t.Helper()
	reqReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request",
		strings.NewReader(`{"email":"`+email+`"}`))
	e.authHandler.RequestOTP(httptest.NewRecorder(), reqReq)

	verifyBody := `{"email":"` + email + `","code":"` + e.mailer.lastCode + `"}`
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	e.authHandler.VerifyOTP(verifyRec, verifyReq)
	return verifyRec.Result().Cookies()[0]
}

// serve wraps next in the real session middleware and drives req through
// it, so next can read auth.UserID(r.Context()).
func (e *testEnv) serve(next http.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.authHandler.Middleware(next).ServeHTTP(rec, req)
	return rec
}

// withURLParam attaches a chi URL param the way the real router would,
// for handlers under test that read chi.URLParam(r, "id") without going
// through an actual chi router.
func withURLParam(req *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

func TestCreate_ValidBody_Returns201(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t, "parent@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2024-03-15","consent":true}`))
	req.AddCookie(cookie)

	rec := env.serve(env.childHandler.Create, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["name"] != "Тимофей" {
		t.Fatalf("name = %v, want Тимофей", body["name"])
	}
	if body["public_share_token"] == "" || body["public_share_token"] == nil {
		t.Fatal("public_share_token is empty, want a generated token")
	}
}

func TestCreate_NoConsent_BadRequest(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t, "parent@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2024-03-15","consent":false}`))
	req.AddCookie(cookie)

	rec := env.serve(env.childHandler.Create, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestCreate_FutureBirthDate_BadRequest(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t, "parent@example.com")

	req := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2099-01-01","consent":true}`))
	req.AddCookie(cookie)

	rec := env.serve(env.childHandler.Create, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestList_ReturnsOnlyOwnChildren(t *testing.T) {
	env := newTestEnv(t)
	cookieA := env.login(t, "a@example.com")
	cookieB := env.login(t, "b@example.com")

	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Child A","birth_date":"2024-03-15","consent":true}`))
	createReq.AddCookie(cookieA)
	env.serve(env.childHandler.Create, createReq)

	listReq := httptest.NewRequest(http.MethodGet, "/api/children", nil)
	listReq.AddCookie(cookieB)
	rec := env.serve(env.childHandler.List, listReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var children []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &children); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(children) != 0 {
		t.Fatalf("children for B = %+v, want empty (A's child must not leak)", children)
	}
}

func TestUpdate_OwnChild_Returns200(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t, "parent@example.com")

	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2024-03-15","consent":true}`))
	createReq.AddCookie(cookie)
	createRec := env.serve(env.childHandler.Create, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	id := int64(created["id"].(float64))

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/children/"+strconv.FormatInt(id, 10),
		strings.NewReader(`{"name":"Тимур"}`))
	patchReq.AddCookie(cookie)
	patchReq = withURLParam(patchReq, "id", strconv.FormatInt(id, 10))
	rec := env.serve(env.childHandler.Update, patchReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var updated map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if updated["name"] != "Тимур" {
		t.Fatalf("name = %v, want Тимур", updated["name"])
	}
}

func TestUpdate_OtherParentsChild_NotFound(t *testing.T) {
	env := newTestEnv(t)
	cookieA := env.login(t, "a@example.com")
	cookieB := env.login(t, "b@example.com")

	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Child A","birth_date":"2024-03-15","consent":true}`))
	createReq.AddCookie(cookieA)
	createRec := env.serve(env.childHandler.Create, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	id := int64(created["id"].(float64))

	patchReq := httptest.NewRequest(http.MethodPatch, "/api/children/"+strconv.FormatInt(id, 10),
		strings.NewReader(`{"name":"Hijacked"}`))
	patchReq.AddCookie(cookieB)
	patchReq = withURLParam(patchReq, "id", strconv.FormatInt(id, 10))
	rec := env.serve(env.childHandler.Update, patchReq)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestDelete_OwnChild_Returns200(t *testing.T) {
	env := newTestEnv(t)
	cookie := env.login(t, "parent@example.com")

	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2024-03-15","consent":true}`))
	createReq.AddCookie(cookie)
	createRec := env.serve(env.childHandler.Create, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	id := int64(created["id"].(float64))

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/children/"+strconv.FormatInt(id, 10), nil)
	deleteReq.AddCookie(cookie)
	deleteReq = withURLParam(deleteReq, "id", strconv.FormatInt(id, 10))
	rec := env.serve(env.childHandler.Delete, deleteReq)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestDelete_OtherParentsChild_NotFound(t *testing.T) {
	env := newTestEnv(t)
	cookieA := env.login(t, "a@example.com")
	cookieB := env.login(t, "b@example.com")

	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Child A","birth_date":"2024-03-15","consent":true}`))
	createReq.AddCookie(cookieA)
	createRec := env.serve(env.childHandler.Create, createReq)
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	id := int64(created["id"].(float64))

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/children/"+strconv.FormatInt(id, 10), nil)
	deleteReq.AddCookie(cookieB)
	deleteReq = withURLParam(deleteReq, "id", strconv.FormatInt(id, 10))
	rec := env.serve(env.childHandler.Delete, deleteReq)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
