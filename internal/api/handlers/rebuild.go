package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

// RebuildJobResponse is returned after queueing a rollup rebuild.
type RebuildJobResponse struct {
	Job QueuedJobResponse `json:"job"`
}

// RebuildJobConflictResponse is returned when a rebuild of this type is already pending.
type RebuildJobConflictResponse struct {
	Error   string        `json:"error"`
	JobType model.JobType `json:"job_type,omitempty"`
}

type rollupRebuild struct {
	store   store.Store
	jobType model.JobType
	create  func(context.Context) (*model.Job, error)
}

func (h rollupRebuild) serve(w http.ResponseWriter, r *http.Request) {
	if err := decodeJSONBody(r.Body, &struct{}{}, jsonBodyOptions{disallowUnknown: true, rejectTrailing: true}); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: errInvalidJSONBody})
		return
	}

	ctx := r.Context()

	active, err := h.store.PendingIngestJob(ctx, h.jobType)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCheckActiveRebuild})
		return
	}

	if active {
		writeJSON(w, http.StatusConflict, h.conflict())

		return
	}

	job, err := h.create(ctx)
	if err != nil {
		if errors.Is(err, store.ErrActiveIngestConflict) {
			writeJSON(w, http.StatusConflict, h.conflict())

			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCreateRebuildJob})

		return
	}

	writeJSON(w, http.StatusCreated, RebuildJobResponse{
		Job: QueuedJobResponse{
			ID:     job.ID,
			Type:   job.Type,
			Status: job.Status,
		},
	})
}

func (h rollupRebuild) conflict() RebuildJobConflictResponse {
	return RebuildJobConflictResponse{
		Error:   errActiveRebuildJob,
		JobType: h.jobType,
	}
}
