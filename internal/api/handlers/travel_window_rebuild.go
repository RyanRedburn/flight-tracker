package handlers

import (
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

// RebuildTravelWindowsHandler queues a full travel-window rollup rebuild.
type RebuildTravelWindowsHandler struct {
	rollupRebuild
}

func NewRebuildTravelWindowsHandler(s store.Store) *RebuildTravelWindowsHandler {
	return &RebuildTravelWindowsHandler{rollupRebuild{
		store:   s,
		jobType: model.JobTypeRebuildRouteTravelWindows,
		create:  s.CreateRebuildRouteTravelWindowsJob,
	}}
}

// Create queues a full rebuild of travel-window rollups from flight_performance.
//
//	@Summary		Queue travel-window rollup rebuild
//	@Description	Queues a full replace of route_travel_window_scopes and route_travel_window_buckets from flight_performance. The body must be empty. Returns 409 when a rebuild of this type is already pending. A running rebuild does not block a new pending job.
//	@Tags			rebuild,internal
//	@Produce		json
//	@Success		201	{object}	RebuildJobResponse
//	@Failure		400	{object}	respond.ErrorResponse
//	@Failure		401	{object}	respond.ErrorResponse
//	@Failure		403	{object}	respond.ErrorResponse
//	@Failure		409	{object}	RebuildJobConflictResponse
//	@Failure		429	{object}	respond.ErrorResponse
//	@Failure		500	{object}	respond.ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/internal/rebuild/travel-windows [post]
func (h *RebuildTravelWindowsHandler) Create(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r)
}
