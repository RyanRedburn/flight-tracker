package handlers

import (
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/api/respond"
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
	respond.WriteJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

// Readiness reports whether the service can accept traffic.
//
//	@Summary		Readiness probe
//	@Description	Returns ready when the database is reachable; otherwise returns 503.
//	@Tags			health,internal
//	@Produce		json
//	@Success		200	{object}	StatusResponse
//	@Failure		503	{object}	respond.ErrorResponse
//	@Router			/ready [get]
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if err := h.store.Ping(r.Context()); err != nil {
		respond.WriteJSON(w, http.StatusServiceUnavailable, respond.ErrorResponse{Error: "database unavailable"})
		return
	}

	respond.WriteJSON(w, http.StatusOK, StatusResponse{Status: "ready"})
}

// DatabaseVersion returns the applied migration version.
//
//	@Summary		Database migration version
//	@Description	Returns the current schema migration version and dirty flag.
//	@Tags			health,internal
//	@Produce		json
//	@Success		200	{object}	store.MigrationVersion
//	@Failure		401	{object}	respond.ErrorResponse
//	@Failure		403	{object}	respond.ErrorResponse
//	@Failure		429	{object}	respond.ErrorResponse
//	@Failure		500	{object}	respond.ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/db/version [get]
func (h *HealthHandler) DatabaseVersion(w http.ResponseWriter, r *http.Request) {
	version, err := h.store.MigrationVersion(r.Context())
	if err != nil {
		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{Error: "database version unavailable"})
		return
	}

	respond.WriteJSON(w, http.StatusOK, version)
}
