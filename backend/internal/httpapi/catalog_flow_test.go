package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"needtobuy/internal/auth"
	"needtobuy/internal/catalog"
	"needtobuy/internal/child"
	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
	"needtobuy/internal/httpapi"
)

func TestCatalogFlow_BrowseWithoutSession_EndToEnd(t *testing.T) {
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

	// No session cookie at all: the catalog must still respond 200 — it's
	// the one domain route in this project with no auth.RequireAuth.
	req := httptest.NewRequest(http.MethodGet, "/api/catalog", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog without session: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected seeded catalog items, got none")
	}

	// Filtered request.
	filteredReq := httptest.NewRequest(http.MethodGet, "/api/catalog?age_range=18m&category=toys", nil)
	filteredRec := httptest.NewRecorder()
	router.ServeHTTP(filteredRec, filteredReq)
	if filteredRec.Code != http.StatusOK {
		t.Fatalf("filtered catalog: status = %d, body = %s", filteredRec.Code, filteredRec.Body.String())
	}
	var filtered []map[string]any
	if err := json.Unmarshal(filteredRec.Body.Bytes(), &filtered); err != nil {
		t.Fatalf("unmarshal filtered: %v", err)
	}
	if len(filtered) == 0 {
		t.Fatal("expected at least one seeded item at 18m/toys")
	}

	// Invalid filter: 400, not a silent empty list.
	badReq := httptest.NewRequest(http.MethodGet, "/api/catalog?category=shoes", nil)
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("invalid category: status = %d, want %d", badRec.Code, http.StatusBadRequest)
	}
}
