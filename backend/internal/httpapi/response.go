package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

// WriteJSON writes v as a JSON response body with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("httpapi: write json: %v", err)
	}
}

type errorEnvelope struct {
	Error *Error `json:"error"`
}

// WriteError writes err as the standard {"error": {...}} JSON envelope.
func WriteError(w http.ResponseWriter, err *Error) {
	if err == nil {
		err = Internal("internal server error")
	}
	if err.HTTPStatus < 100 {
		err.HTTPStatus = http.StatusInternalServerError
	}
	WriteJSON(w, err.HTTPStatus, errorEnvelope{Error: err})
}
