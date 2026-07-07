package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/ingest"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type IngestHandler struct {
	store           store.Store
	maxIngestMonths int
}

func NewIngestHandler(s store.Store, maxIngestMonths int) *IngestHandler {
	return &IngestHandler{store: s, maxIngestMonths: maxIngestMonths}
}

type ingestJobResponse struct {
	ID     string          `json:"id"`
	Year   int             `json:"year"`
	Month  int             `json:"month"`
	Status model.JobStatus `json:"status"`
}

type ingestResponse struct {
	Jobs            []ingestJobResponse `json:"jobs"`
	MonthsRequested int                 `json:"months_requested"`
}

func (h *IngestHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{jsonErrKey: "invalid json body"})
		return
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{jsonErrKey: err.Error()})
		return
	}

	months, err := ingest.ExpandMonths(ingest.RangeInput{
		StartYear:  req.StartYear,
		StartMonth: req.StartMonth,
		EndYear:    req.EndYear,
		EndMonth:   req.EndMonth,
	}, h.maxIngestMonths)
	if err != nil {
		h.writeRangeError(w, err)
		return
	}

	ctx := r.Context()

	active, err := h.store.ActiveBTSIngestMonths(ctx, months)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to check active ingest jobs"})
		return
	}

	if len(active) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{
			jsonErrKey:             "ingest jobs already pending or running for one or more requested months",
			"active_ingest_months": active,
		})

		return
	}

	if !req.Force {
		existing, err := h.store.MonthsWithOnTimeFlightData(ctx, months)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to check existing flight data"})
			return
		}

		if len(existing) > 0 {
			writeJSON(w, http.StatusConflict, map[string]any{
				jsonErrKey:             "flight data already exists for one or more requested months; set force=true to re-import",
				"existing_data_months": existing,
			})

			return
		}
	}

	jobs := make([]ingestJobResponse, 0, len(months))

	for _, ym := range months {
		job, err := h.store.CreateBTSIngestJob(ctx, ym.Year, ym.Month)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to create ingest job"})
			return
		}

		jobs = append(jobs, ingestJobResponse{
			ID:     job.ID,
			Year:   ym.Year,
			Month:  ym.Month,
			Status: job.Status,
		})
	}

	writeJSON(w, http.StatusCreated, ingestResponse{
		Jobs:            jobs,
		MonthsRequested: len(months),
	})
}

func (h *IngestHandler) writeRangeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ingest.ErrRangeTooLarge):
		writeJSON(w, http.StatusBadRequest, map[string]string{
			jsonErrKey: "requested range exceeds maximum allowed months",
		})
	case errors.Is(err, ingest.ErrEndBeforeStart):
		writeJSON(w, http.StatusBadRequest, map[string]string{
			jsonErrKey: "end date must be on or after start date",
		})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{jsonErrKey: err.Error()})
	}
}
