package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type HealthHandler struct {
	store store.Store
}

func NewHealthHandler(s store.Store) *HealthHandler {
	return &HealthHandler{store: s}
}

func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *HealthHandler) DatabaseVersion(w http.ResponseWriter, r *http.Request) {
	version, err := h.store.MigrationVersion(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database version unavailable")
		return
	}

	writeJSON(w, http.StatusOK, version)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
