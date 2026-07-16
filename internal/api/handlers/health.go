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

// Liveness reports that the process is running.
//
//	@Summary		Liveness probe
//	@Description	Returns ok when the HTTP server is up. Does not check the database.
//	@Tags			health,internal
//	@Produce		json
//	@Success		200	{object}	StatusResponse
//	@Router			/health [get]
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

// Readiness reports whether the service can accept traffic.
//
//	@Summary		Readiness probe
//	@Description	Returns ready when the database is reachable; otherwise returns 503.
//	@Tags			health,internal
//	@Produce		json
//	@Success		200	{object}	StatusResponse
//	@Failure		503	{object}	ErrorResponse
//	@Router			/ready [get]
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{Error: "database unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, StatusResponse{Status: "ready"})
}

// DatabaseVersion returns the applied migration version.
//
//	@Summary		Database migration version
//	@Description	Returns the current schema migration version and dirty flag.
//	@Tags			health,internal
//	@Produce		json
//	@Success		200	{object}	store.MigrationVersion
//	@Failure		500	{object}	ErrorResponse
//	@Router			/db/version [get]
func (h *HealthHandler) DatabaseVersion(w http.ResponseWriter, r *http.Request) {
	version, err := h.store.MigrationVersion(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "database version unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, version)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
