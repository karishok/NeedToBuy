package catalog

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"needtobuy/internal/dbtest"
)

func TestList_NoFilters_ReturnsSeedData(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected seeded catalog items, got none")
	}
}

func TestList_InvalidAgeRange_BadRequest(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog?age_range=not-a-code", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestList_InvalidCategory_BadRequest(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog?category=shoes", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestList_NoFilters_ImageURLIsSetInResponse(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected seeded catalog items, got none")
	}
	for _, item := range items {
		// Migration 000007 back-fills a placeholder photo onto every seed
		// row, so the "image_url" key must be present and non-null here.
		// (The itemResponse type still emits "image_url": null for any
		// future item that genuinely has no photo — this test just isn't
		// exercising that case, since no seed row is in that state anymore.)
		imageURL, ok := item["image_url"]
		if !ok {
			t.Fatalf("item %v missing image_url key entirely", item)
		}
		if imageURL == nil || imageURL == "" {
			t.Fatalf("item image_url = %v, want a non-empty placeholder", imageURL)
		}
	}
}

func TestList_ValidFilters_AppliesBoth(t *testing.T) {
	tx := dbtest.Tx(t)
	h := NewHandler(tx)

	req := httptest.NewRequest(http.MethodGet, "/api/catalog?age_range=18m&category=toys", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var items []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one seeded item at 18m/toys")
	}
	for _, item := range items {
		if item["age_range_code"] != "18m" {
			t.Fatalf("item age_range_code = %v, want 18m", item["age_range_code"])
		}
		if item["category"] != "toys" {
			t.Fatalf("item category = %v, want toys", item["category"])
		}
	}
}
