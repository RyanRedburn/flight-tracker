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

// IngestJobResponse is one queued flight-schedule ingest job.
type IngestJobResponse struct {
	ID     string          `json:"id"`
	Year   int             `json:"year"`
	Month  int             `json:"month"`
	Status model.JobStatus `json:"status"`
}

// IngestResponse is returned after successfully queueing flight-schedule ingest jobs.
type IngestResponse struct {
	Jobs            []IngestJobResponse `json:"jobs"`
	MonthsRequested int                 `json:"months_requested"`
}

// Create queues flight schedule data ingest jobs for a month range.
//
//	@Summary		Queue flight schedule data ingest
//	@Description	Queues one import job per month in the requested range. Omit end_year/end_month for a single month. Set force=true to re-import months that already have data.
//	@Tags			ingest,internal
//	@Accept			json
//	@Produce		json
//	@Param			body	body		model.IngestRequest	true	"Ingest range"
//	@Success		201		{object}	IngestResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	FlightScheduleIngestConflictResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/v1/ingest [post]
func (h *IngestHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: "invalid json body"})
		return
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
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
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to check active ingest jobs"})
		return
	}

	if len(active) > 0 {
		writeJSON(w, http.StatusConflict, FlightScheduleIngestConflictResponse{
			Error:              "ingest jobs already pending or running for one or more requested months",
			ActiveIngestMonths: active,
		})

		return
	}

	if !req.Force {
		existing, err := h.store.MonthsWithOnTimeFlightData(ctx, months)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to check existing flight data"})
			return
		}

		if len(existing) > 0 {
			writeJSON(w, http.StatusConflict, FlightScheduleIngestConflictResponse{
				Error:              "flight data already exists for one or more requested months; set force=true to re-import",
				ExistingDataMonths: existing,
			})

			return
		}
	}

	jobs := make([]IngestJobResponse, 0, len(months))

	for _, ym := range months {
		job, err := h.store.CreateBTSIngestJob(ctx, ym.Year, ym.Month)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to create ingest job"})
			return
		}

		jobs = append(jobs, IngestJobResponse{
			ID:     job.ID,
			Year:   ym.Year,
			Month:  ym.Month,
			Status: job.Status,
		})
	}

	writeJSON(w, http.StatusCreated, IngestResponse{
		Jobs:            jobs,
		MonthsRequested: len(months),
	})
}

func (h *IngestHandler) writeRangeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ingest.ErrRangeTooLarge):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "requested range exceeds maximum allowed months",
		})
	case errors.Is(err, ingest.ErrEndBeforeStart):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{
			Error: "end date must be on or after start date",
		})
	default:
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	}
}
