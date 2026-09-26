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

// IngestJobResponse is one queued flight-performance ingest job.
type IngestJobResponse struct {
	ID     string          `json:"id"`
	Year   int             `json:"year"`
	Month  int             `json:"month"`
	Status model.JobStatus `json:"status"`
}

// IngestResponse is returned after successfully queueing flight-performance ingest jobs.
type IngestResponse struct {
	Jobs            []IngestJobResponse `json:"jobs"`
	MonthsRequested int                 `json:"months_requested"`
}

// Create queues flight performance data ingest jobs for a month range.
//
//	@Summary		Queue flight performance data ingest
//	@Description	Queues one import job per month in the requested range. Omit end_year/end_month for a single month. Set force=true to re-import months that already have data.
//	@Tags			ingest,internal
//	@Accept			json
//	@Produce		json
//	@Param			body	body		model.IngestRequest	true	"Ingest range"
//	@Success		201		{object}	IngestResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		409		{object}	MonthIngestConflictResponse
//	@Failure		429		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/ingest [post]
func (h *IngestHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.IngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: errInvalidJSONBody})
		return
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	queued, ok := queueMonthIngestJobs(
		w,
		r,
		h.maxIngestMonths,
		monthIngestInput{
			startYear:  req.StartYear,
			startMonth: req.StartMonth,
			endYear:    req.EndYear,
			endMonth:   req.EndMonth,
			force:      req.Force,
		},
		h.store.ActiveFlightPerformanceIngestMonths,
		h.store.MonthsWithFlightPerformanceData,
		errFailedCheckExistingFlight,
		"flight data already exists for one or more requested months; set force=true to re-import",
		h.store.CreateFlightPerformanceIngestJob,
	)
	if !ok {
		return
	}

	jobs := make([]IngestJobResponse, 0, len(queued))
	for _, job := range queued {
		jobs = append(jobs, IngestJobResponse(job))
	}

	writeJSON(w, http.StatusCreated, IngestResponse{
		Jobs:            jobs,
		MonthsRequested: len(jobs),
	})
}

func writeIngestRangeError(w http.ResponseWriter, err error) {
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
