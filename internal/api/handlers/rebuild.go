package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/api/respond"
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
		respond.WriteJSON(w, http.StatusBadRequest, respond.ErrorResponse{Error: errInvalidJSONBody})
		return
	}

	ctx := r.Context()

	active, err := h.store.PendingIngestJob(ctx, h.jobType)
	if err != nil {
		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{Error: errFailedCheckActiveRebuild})
		return
	}

	if active {
		respond.WriteJSON(w, http.StatusConflict, h.conflict())

		return
	}

	job, err := h.create(ctx)
	if err != nil {
		if errors.Is(err, store.ErrActiveIngestConflict) {
			respond.WriteJSON(w, http.StatusConflict, h.conflict())

			return
		}

		respond.WriteJSON(w, http.StatusInternalServerError, respond.ErrorResponse{Error: errFailedCreateRebuildJob})

		return
	}

	respond.WriteJSON(w, http.StatusCreated, RebuildJobResponse{
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
