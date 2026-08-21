package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"needtobuy/internal/auth"
	"needtobuy/internal/catalog"
	"needtobuy/internal/child"
	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
	"needtobuy/internal/httpapi"
)

func TestChildFlow_CreateListUpdateDelete_EndToEnd(t *testing.T) {
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
	catalogHandler := catalog.NewHandler(conn)
	router := httpapi.NewRouter(conn, authHandler, childHandler, catalogHandler)
	email := fmt.Sprintf("child-e2e-%d@example.com", time.Now().UnixNano())

	// Log in.
	reqBody := fmt.Sprintf(`{"email":%q}`, email)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/otp/request", strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("otp request: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	verifyBody := fmt.Sprintf(`{"email":%q,"code":%q}`, email, mailer.lastCode)
	verifyReq := httptest.NewRequest(http.MethodPost, "/api/auth/otp/verify", strings.NewReader(verifyBody))
	verifyRec := httptest.NewRecorder()
	router.ServeHTTP(verifyRec, verifyReq)
	if verifyRec.Code != http.StatusOK {
		t.Fatalf("otp verify: status = %d, body = %s", verifyRec.Code, verifyRec.Body.String())
	}
	sessionCookie := verifyRec.Result().Cookies()[0]

	// No cookie at all: creating a child must be rejected.
	noCookieReq := httptest.NewRequest(http.MethodPost, "/api/children", strings.NewReader(`{"name":"x","birth_date":"2024-01-01","consent":true}`))
	noCookieRec := httptest.NewRecorder()
	router.ServeHTTP(noCookieRec, noCookieReq)
	if noCookieRec.Code != http.StatusUnauthorized {
		t.Fatalf("create without cookie: status = %d, want %d", noCookieRec.Code, http.StatusUnauthorized)
	}

	// Create a child.
	createReq := httptest.NewRequest(http.MethodPost, "/api/children",
		strings.NewReader(`{"name":"Тимофей","birth_date":"2000-01-01","consent":true}`))
	createReq.AddCookie(sessionCookie)
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body = %s", createRec.Code, createRec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response: %v", err)
	}
	id := int64(created["id"].(float64))
	if created["birth_date"] != "2000-01-01" {
		t.Fatalf("birth_date = %v, want 2000-01-01", created["birth_date"])
	}
	if created["age_range_code"] != "12y+" {
		t.Fatalf("age_range_code = %v, want 12y+", created["age_range_code"])
	}

	// List: the new child appears.
	listReq := httptest.NewRequest(http.MethodGet, "/api/children", nil)
	listReq.AddCookie(sessionCookie)
	listRec := httptest.NewRecorder()
	router.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list: status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	var listed []map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("unmarshal list response: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("list = %+v, want exactly 1 child", listed)
	}

	// Update the child's name.
	patchReq := httptest.NewRequest(http.MethodPatch, fmt.Sprintf("/api/children/%d", id),
		strings.NewReader(`{"name":"Тимур"}`))
	patchReq.AddCookie(sessionCookie)
	patchRec := httptest.NewRecorder()
	router.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("patch: status = %d, body = %s", patchRec.Code, patchRec.Body.String())
	}

	// Delete the child.
	deleteReq := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/children/%d", id), nil)
	deleteReq.AddCookie(sessionCookie)
	deleteRec := httptest.NewRecorder()
	router.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("delete: status = %d, body = %s", deleteRec.Code, deleteRec.Body.String())
	}

	// List again: empty.
	listReq2 := httptest.NewRequest(http.MethodGet, "/api/children", nil)
	listReq2.AddCookie(sessionCookie)
	listRec2 := httptest.NewRecorder()
	router.ServeHTTP(listRec2, listReq2)
	var listed2 []map[string]any
	if err := json.Unmarshal(listRec2.Body.Bytes(), &listed2); err != nil {
		t.Fatalf("unmarshal second list response: %v", err)
	}
	if len(listed2) != 0 {
		t.Fatalf("list after delete = %+v, want empty", listed2)
	}
}
