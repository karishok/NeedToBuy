// Package apierr defines the JSON error envelope shared by every HTTP
// handler package. It has no internal dependencies so both httpapi and
// domain packages like auth can import it without creating a cycle.
package apierr

import "net/http"

// Error is an application error that maps directly to an HTTP response.
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
}

func (e *Error) Error() string { return e.Message }

// NotFound builds a 404 error for a missing resource named what
// (e.g. "child").
func NotFound(what string) *Error {
	return &Error{Code: "not_found", Message: what + " not found", HTTPStatus: http.StatusNotFound}
}

// BadRequest builds a 400 error with the given human-readable message.
func BadRequest(message string) *Error {
	return &Error{Code: "bad_request", Message: message, HTTPStatus: http.StatusBadRequest}
}

// Internal builds a 500 error with the given human-readable message.
func Internal(message string) *Error {
	return &Error{Code: "internal", Message: message, HTTPStatus: http.StatusInternalServerError}
}
