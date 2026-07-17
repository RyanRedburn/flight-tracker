package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/api/query"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/go-chi/chi/v5"
)

type JobsHandler struct {
	store store.Store
}

func NewJobsHandler(s store.Store) *JobsHandler {
	return &JobsHandler{store: s}
}

// JobResponse is a background job as returned by the jobs API.
type JobResponse struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Status    model.JobStatus `json:"status"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	Year      *int            `json:"year,omitempty"`
	Month     *int            `json:"month,omitempty"`
	CreatedAt string          `json:"created_at"`
	UpdatedAt string          `json:"updated_at"`
	StartedAt *string         `json:"started_at,omitempty"`
	EndedAt   *string         `json:"ended_at,omitempty"`
}

// Get returns a single job by ID.
//
//	@Summary		Get job by ID
//	@Description	Returns status and details for a background job. Flight-schedule jobs include year and month when available.
//	@Tags			jobs,internal
//	@Produce		json
//	@Param			id	path		string	true	"Job ID"
//	@Success		200	{object}	JobResponse
//	@Failure		400	{object}	ErrorResponse
//	@Failure		404	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Router			/api/v1/jobs/{id} [get]
func (h *JobsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := query.ParseJobID(id); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	job, err := h.store.GetJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "job not found"})
			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to get job"})

		return
	}

	resp, err := h.toJobResponse(r.Context(), job)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to load job details"})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// List returns recent jobs.
//
//	@Summary		List jobs
//	@Description	Returns the most recent background jobs, newest first.
//	@Tags			jobs,internal
//	@Produce		json
//	@Param			limit	query		int	false	"Max jobs to return (1-500, default 50)"
//	@Success		200		{array}		JobResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/v1/jobs [get]
func (h *JobsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, err := query.ParseJobsList(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	jobs, err := h.store.ListJobs(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to list jobs"})
		return
	}

	if jobs == nil {
		writeJSON(w, http.StatusOK, []JobResponse{})
		return
	}

	responses := make([]JobResponse, 0, len(jobs))
	for _, job := range jobs {
		resp, err := h.toJobResponse(r.Context(), job)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to load job details"})
			return
		}

		responses = append(responses, resp)
	}

	writeJSON(w, http.StatusOK, responses)
}

func (h *JobsHandler) toJobResponse(ctx context.Context, job *model.Job) (JobResponse, error) {
	resp := JobResponse{
		ID:        job.ID,
		Type:      job.Type,
		Status:    job.Status,
		Result:    job.Result,
		Error:     job.Error,
		CreatedAt: job.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: job.UpdatedAt.UTC().Format(time.RFC3339),
	}

	if job.StartedAt != nil {
		started := job.StartedAt.UTC().Format(time.RFC3339)
		resp.StartedAt = &started
	}

	if job.EndedAt != nil {
		ended := job.EndedAt.UTC().Format(time.RFC3339)
		resp.EndedAt = &ended
	}

	if job.Type != model.JobTypeImportFlightPerformance {
		return resp, nil
	}

	detail, err := h.store.GetFlightPerformanceIngestJob(ctx, job.ID)
	if err != nil {
		return JobResponse{}, err
	}

	year := detail.Year
	month := detail.Month
	resp.Year = &year
	resp.Month = &month

	return resp, nil
}
