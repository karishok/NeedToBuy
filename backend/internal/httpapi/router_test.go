package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"needtobuy/internal/auth"
	"needtobuy/internal/child"
	"needtobuy/internal/db"
	"needtobuy/internal/dbtest"
	"needtobuy/internal/httpapi"
)

type noopMailer struct{}

func (noopMailer) SendOTP(_ context.Context, _ string, _ string) error { return nil }

func TestHealthz_OK(t *testing.T) {
	dsn := dbtest.DSN(t)

	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer conn.Close()

	authHandler := auth.NewHandler(conn, noopMailer{}, "test-pepper")
	childHandler := child.NewHandler(conn)
	router := httpapi.NewRouter(conn, authHandler, childHandler)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

func TestHealthz_DatabaseDown(t *testing.T) {
	dsn := dbtest.DSN(t)

	conn, err := db.Connect(dsn)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	authHandler := auth.NewHandler(conn, noopMailer{}, "test-pepper")
	childHandler := child.NewHandler(conn)
	conn.Close() // force the ping in the handler to fail

	router := httpapi.NewRouter(conn, authHandler, childHandler)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
}
