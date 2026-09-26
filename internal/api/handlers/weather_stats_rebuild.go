package handlers

import (
	"errors"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

// RebuildWeatherStatsHandler queues a full route weather-category rollup rebuild.
type RebuildWeatherStatsHandler struct {
	store store.Store
}

// RebuildWeatherStatsResponse is returned after queueing a weather-stats rebuild.
type RebuildWeatherStatsResponse struct {
	Job QueuedJobResponse `json:"job"`
}

// RebuildWeatherStatsConflictResponse is returned when a rebuild job is already pending or running.
type RebuildWeatherStatsConflictResponse struct {
	Error   string        `json:"error"`
	JobType model.JobType `json:"job_type,omitempty"`
}

func NewRebuildWeatherStatsHandler(s store.Store) *RebuildWeatherStatsHandler {
	return &RebuildWeatherStatsHandler{store: s}
}

// Create queues a full rebuild of route weather-category rollups.
//
//	@Summary		Queue route weather-stats rollup rebuild
//	@Description	Queues a full replace of route_weather_category_buckets from flight_performance and classified weather_observations. Nearest observation within ±30 minutes of scheduled departure (origin CRS local time) and scheduled arrival (that departure plus crs_elapsed_time minutes). The body must be empty. Returns 409 when a rebuild job is already pending or running.
//	@Tags			rebuild,internal
//	@Produce		json
//	@Success		201	{object}	RebuildWeatherStatsResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		409	{object}	RebuildWeatherStatsConflictResponse
//	@Failure		429	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/rebuild/weather-stats [post]
func (h *RebuildWeatherStatsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := decodeJSONBody(r.Body, &struct{}{}, jsonBodyOptions{disallowUnknown: true, rejectTrailing: true}); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: errInvalidJSONBody})
		return
	}

	ctx := r.Context()

	active, err := h.store.ActiveIngestJob(ctx, model.JobTypeRebuildRouteWeatherStats)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCheckActiveRebuild})
		return
	}

	if active {
		writeJSON(w, http.StatusConflict, rebuildWeatherStatsConflict())

		return
	}

	job, err := h.store.CreateRebuildRouteWeatherStatsJob(ctx)
	if err != nil {
		if errors.Is(err, store.ErrActiveIngestConflict) {
			writeJSON(w, http.StatusConflict, rebuildWeatherStatsConflict())

			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCreateRebuildJob})

		return
	}

	writeJSON(w, http.StatusCreated, RebuildWeatherStatsResponse{
		Job: QueuedJobResponse{
			ID:     job.ID,
			Type:   job.Type,
			Status: job.Status,
		},
	})
}

func rebuildWeatherStatsConflict() RebuildWeatherStatsConflictResponse {
	return RebuildWeatherStatsConflictResponse{
		Error:   errActiveRebuildJob,
		JobType: model.JobTypeRebuildRouteWeatherStats,
	}
}
