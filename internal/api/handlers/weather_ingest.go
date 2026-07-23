package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/ingest"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type WeatherIngestHandler struct {
	store           store.Store
	maxIngestMonths int
}

func NewWeatherIngestHandler(s store.Store, maxIngestMonths int) *WeatherIngestHandler {
	return &WeatherIngestHandler{store: s, maxIngestMonths: maxIngestMonths}
}

// WeatherIngestJobResponse is one queued weather observation ingest job.
type WeatherIngestJobResponse struct {
	ID       string          `json:"id"`
	Year     int             `json:"year"`
	Month    int             `json:"month"`
	Stations []string        `json:"stations"`
	Status   model.JobStatus `json:"status"`
}

// WeatherIngestResponse is returned after successfully queueing weather ingest jobs.
type WeatherIngestResponse struct {
	Jobs            []WeatherIngestJobResponse `json:"jobs"`
	MonthsRequested int                        `json:"months_requested"`
}

// Create queues weather observation ingest jobs for a month range and station list.
//
//	@Summary		Queue weather observation data ingest
//	@Description	Queues one import job per month in the requested range for the given IEM station ids. Omit end_year/end_month for a single month. Set force=true to re-import months that already have data.
//	@Tags			ingest,internal
//	@Accept			json
//	@Produce		json
//	@Param			body	body		model.WeatherIngestRequest	true	"Ingest range and stations"
//	@Success		201		{object}	WeatherIngestResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	WeatherIngestConflictResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/v1/ingest/weather [post]
func (h *WeatherIngestHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.WeatherIngestRequest
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
		writeIngestRangeError(w, err)
		return
	}

	ctx := r.Context()

	active, err := h.store.ActiveWeatherIngestMonths(ctx, months)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to check active ingest jobs"})
		return
	}

	if len(active) > 0 {
		writeJSON(w, http.StatusConflict, WeatherIngestConflictResponse{
			Error:              "ingest jobs already pending or running for one or more requested months",
			ActiveIngestMonths: active,
		})

		return
	}

	if !req.Force {
		existing, err := h.store.MonthsWithWeatherData(ctx, months)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to check existing weather data"})
			return
		}

		if len(existing) > 0 {
			writeJSON(w, http.StatusConflict, WeatherIngestConflictResponse{
				Error:              "weather data already exists for one or more requested months; set force=true to re-import",
				ExistingDataMonths: existing,
			})

			return
		}
	}

	jobs := make([]WeatherIngestJobResponse, 0, len(months))

	for _, ym := range months {
		job, err := h.store.CreateWeatherIngestJob(ctx, ym.Year, ym.Month, req.Stations)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to create ingest job"})
			return
		}

		jobs = append(jobs, WeatherIngestJobResponse{
			ID:       job.ID,
			Year:     ym.Year,
			Month:    ym.Month,
			Stations: append([]string(nil), req.Stations...),
			Status:   job.Status,
		})
	}

	writeJSON(w, http.StatusCreated, WeatherIngestResponse{
		Jobs:            jobs,
		MonthsRequested: len(months),
	})
}
