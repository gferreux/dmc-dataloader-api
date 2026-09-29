// Package httpx writes the console's JSON responses.
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorBody is the error envelope shared with the Angular console.
type ErrorBody struct {
	Error ErrorPayload `json:"error"`
}

// ErrorPayload is one API error.
type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details"`
}

// WriteJSON writes a JSON response.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("writing response", "error", err)
	}
}

// WriteError writes the standard error envelope. Nil details become an empty array.
func WriteError(w http.ResponseWriter, status int, code, message string, details any) {
	if details == nil {
		details = []any{}
	}

	WriteJSON(w, status, ErrorBody{Error: ErrorPayload{
		Code:    code,
		Message: message,
		Details: details,
	}})
}
