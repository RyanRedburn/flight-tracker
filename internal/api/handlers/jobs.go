package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/operator"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/go-chi/chi/v5"
)

type JobsHandler struct {
	store  store.Store
	worker *operator.Worker
}

func NewJobsHandler(s store.Store, worker *operator.Worker) *JobsHandler {
	return &JobsHandler{store: s, worker: worker}
}

func (h *JobsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	job, err := h.store.GetJob(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
			return
		}

		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get job"})

		return
	}

	writeJSON(w, http.StatusOK, job)
}

func (h *JobsHandler) List(w http.ResponseWriter, r *http.Request) {
	limit := 50

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
			return
		}

		limit = parsed
	}

	jobs, err := h.store.ListJobs(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list jobs"})
		return
	}

	if jobs == nil {
		jobs = []*model.Job{}
	}

	writeJSON(w, http.StatusOK, jobs)
}
