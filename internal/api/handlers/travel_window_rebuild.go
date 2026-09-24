package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

// RebuildTravelWindowsHandler queues a full travel-window rollup rebuild.
type RebuildTravelWindowsHandler struct {
	store store.Store
}

// RebuildTravelWindowsJobResponse is the queued rebuild job.
type RebuildTravelWindowsJobResponse struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Status model.JobStatus `json:"status"`
}

// RebuildTravelWindowsResponse is returned after queueing a travel-window rebuild.
type RebuildTravelWindowsResponse struct {
	Job RebuildTravelWindowsJobResponse `json:"job"`
}

// RebuildTravelWindowsConflictResponse is returned when a rebuild job is already pending or running.
type RebuildTravelWindowsConflictResponse struct {
	Error   string `json:"error"`
	JobType string `json:"job_type,omitempty"`
}

func NewRebuildTravelWindowsHandler(s store.Store) *RebuildTravelWindowsHandler {
	return &RebuildTravelWindowsHandler{store: s}
}

// Create queues a full rebuild of travel-window rollups from flight_performance.
//
//	@Summary		Queue travel-window rollup rebuild
//	@Description	Queues a full replace of route_travel_window_scopes and route_travel_window_buckets from flight_performance. The body must be empty. Returns 409 when a rebuild job is already pending or running.
//	@Tags			rebuild,internal
//	@Produce		json
//	@Success		201	{object}	RebuildTravelWindowsResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		409	{object}	RebuildTravelWindowsConflictResponse
//	@Failure		429	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/rebuild/travel-windows [post]
func (h *RebuildTravelWindowsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := decodeEmptyBody(r.Body); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: errInvalidJSONBody})
		return
	}

	ctx := r.Context()

	active, err := h.store.ActiveIngestJob(ctx, model.JobTypeRebuildRouteTravelWindows)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCheckActiveRebuild})
		return
	}

	if active {
		writeJSON(w, http.StatusConflict, rebuildTravelWindowsConflict())

		return
	}

	job, err := h.store.CreateRebuildRouteTravelWindowsJob(ctx)
	if err != nil {
		if errors.Is(err, store.ErrActiveIngestConflict) {
			writeJSON(w, http.StatusConflict, rebuildTravelWindowsConflict())

			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCreateRebuildJob})

		return
	}

	writeJSON(w, http.StatusCreated, RebuildTravelWindowsResponse{
		Job: RebuildTravelWindowsJobResponse{
			ID:     job.ID,
			Type:   job.Type,
			Status: job.Status,
		},
	})
}

func rebuildTravelWindowsConflict() RebuildTravelWindowsConflictResponse {
	return RebuildTravelWindowsConflictResponse{
		Error:   errActiveRebuildJob,
		JobType: model.JobTypeRebuildRouteTravelWindows,
	}
}

func decodeEmptyBody(body io.Reader) error {
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	var req struct{}
	if err := decoder.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}

		return err
	}

	if err := decoder.Decode(&struct{}{}); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}

		return err
	}

	return errors.New("unexpected trailing json")
}
