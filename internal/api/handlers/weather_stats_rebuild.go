package handlers

import (
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

// RebuildWeatherStatsHandler queues a full route weather-category rollup rebuild.
type RebuildWeatherStatsHandler struct {
	rollupRebuild
}

func NewRebuildWeatherStatsHandler(s store.Store) *RebuildWeatherStatsHandler {
	return &RebuildWeatherStatsHandler{rollupRebuild{
		store:   s,
		jobType: model.JobTypeRebuildRouteWeatherStats,
		create:  s.CreateRebuildRouteWeatherStatsJob,
	}}
}

// Create queues a full rebuild of route weather-category rollups.
//
//	@Summary		Queue route weather-stats rollup rebuild
//	@Description	Queues a full replace of route weather-category rollups from flight performance and weather observations. The body must be empty. Returns 409 when a rebuild of this type is already pending. A running rebuild does not block a new pending job.
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
//	@Router			/api/internal/rebuild/weather-stats [post]
func (h *RebuildWeatherStatsHandler) Create(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r)
}
