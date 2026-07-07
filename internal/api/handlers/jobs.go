package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

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

type jobResponse struct {
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

func (h *JobsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{jsonErrKey: "id is required"})
		return
	}

	job, err := h.store.GetJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{jsonErrKey: "job not found"})
			return
		}

		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to get job"})

		return
	}

	resp, err := h.toJobResponse(r.Context(), job)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to load job details"})
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *JobsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 50

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{jsonErrKey: "invalid limit"})
			return
		}

		limit = parsed
	}

	jobs, err := h.store.ListJobs(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to list jobs"})
		return
	}

	if jobs == nil {
		writeJSON(w, http.StatusOK, []jobResponse{})
		return
	}

	responses := make([]jobResponse, 0, len(jobs))
	for _, job := range jobs {
		resp, err := h.toJobResponse(r.Context(), job)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to load job details"})
			return
		}

		responses = append(responses, resp)
	}

	writeJSON(w, http.StatusOK, responses)
}

func (h *JobsHandler) toJobResponse(ctx context.Context, job *model.Job) (jobResponse, error) {
	resp := jobResponse{
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

	if job.Type != model.JobTypeImportBTSOnTime {
		return resp, nil
	}

	detail, err := h.store.GetBTSIngestJob(ctx, job.ID)
	if err != nil {
		return jobResponse{}, err
	}

	year := detail.Year
	month := detail.Month
	resp.Year = &year
	resp.Month = &month

	return resp, nil
}
