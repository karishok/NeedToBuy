package httpapi_test

import (
	"net/http"
	"testing"

	"needtobuy/internal/httpapi"
)

func TestNotFound(t *testing.T) {
	err := httpapi.NotFound("child")

	if err.Code != "not_found" {
		t.Errorf("Code = %q, want %q", err.Code, "not_found")
	}
	if err.HTTPStatus != http.StatusNotFound {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusNotFound)
	}
	if err.Message != "child not found" {
		t.Errorf("Message = %q, want %q", err.Message, "child not found")
	}
}

func TestBadRequest(t *testing.T) {
	err := httpapi.BadRequest("email is required")

	if err.Code != "bad_request" {
		t.Errorf("Code = %q, want %q", err.Code, "bad_request")
	}
	if err.HTTPStatus != http.StatusBadRequest {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusBadRequest)
	}
}

func TestInternal(t *testing.T) {
	err := httpapi.Internal("database unavailable")

	if err.Code != "internal" {
		t.Errorf("Code = %q, want %q", err.Code, "internal")
	}
	if err.HTTPStatus != http.StatusInternalServerError {
		t.Errorf("HTTPStatus = %d, want %d", err.HTTPStatus, http.StatusInternalServerError)
	}
}
