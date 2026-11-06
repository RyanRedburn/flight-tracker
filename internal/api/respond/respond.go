package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

const (
	ErrUnauthorized         = "unauthorized"
	ErrForbidden            = "forbidden"
	ErrRateLimitExceeded    = "rate limit exceeded"
	ErrAuthUnavailable      = "authentication unavailable"
	ErrRateLimitUnavailable = "rate limit unavailable"
)

// ErrorResponse is the common JSON error body returned by the API.
type ErrorResponse struct {
	Error string `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(payload); err != nil {
		slog.Error("encode json response", "error", err, "status", status)
	}
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, ErrorResponse{Error: msg})
}
