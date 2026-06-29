package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/operator"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type JobsHandler struct {
	store  store.Store
	worker *operator.Worker
}

func NewJobsHandler(s store.Store, worker *operator.Worker) *JobsHandler {
	return &JobsHandler{store: s, worker: worker}
}

type createJobRequest struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type createJobResponse struct {
	ID     string          `json:"id"`
	Status model.JobStatus `json:"status"`
}

func (h *JobsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}

	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required")
		return
	}

	payload := req.Payload

	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}

	now := time.Now().UTC()
	job := &model.Job{
		ID:        uuid.NewString(),
		Type:      req.Type,
		Payload:   payload,
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := h.store.CreateJob(r.Context(), job); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create job")
		return
	}

	h.worker.Submit(job.ID)

	writeJSON(w, http.StatusCreated, createJobResponse{
		ID:     job.ID,
		Status: job.Status,
	})
}

func (h *JobsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	job, err := h.store.GetJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "job not found")
			return
		}

		writeError(w, http.StatusInternalServerError, "failed to get job")

		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (h *JobsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 50

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}

		limit = parsed
	}

	jobs, err := h.store.ListJobs(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list jobs")
		return
	}

	if jobs == nil {
		jobs = []*model.Job{}
	}

	writeJSON(w, http.StatusOK, jobs)
}
