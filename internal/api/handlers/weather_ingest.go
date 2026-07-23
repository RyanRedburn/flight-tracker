package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/ingest"
	"github.com/RyanRedburn/flight-tracker/internal/ingest/iem"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

// WeatherStationResolver builds a default IEM station list when the request omits stations.
type WeatherStationResolver interface {
	Resolve(ctx context.Context) (stations []string, unmatched []string, err error)
}

type WeatherIngestHandler struct {
	store           store.Store
	maxIngestMonths int
	resolver        WeatherStationResolver
	logger          *slog.Logger
}

func NewWeatherIngestHandler(
	s store.Store,
	maxIngestMonths int,
	resolver WeatherStationResolver,
	logger *slog.Logger,
) *WeatherIngestHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &WeatherIngestHandler{
		store:           s,
		maxIngestMonths: maxIngestMonths,
		resolver:        resolver,
		logger:          logger,
	}
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
	Jobs               []WeatherIngestJobResponse `json:"jobs"`
	MonthsRequested    int                        `json:"months_requested"`
	UnresolvedAirports []string                   `json:"unresolved_airports,omitempty"`
}

// Create queues weather observation ingest jobs for a month range.
//
//	@Summary		Queue weather observation data ingest
//	@Description	Queues one import job per month in the requested range. Provide stations explicitly, or omit stations to resolve from distinct BTS origin/dest codes intersected with US IEM ASOS metadata. Omit end_year/end_month for a single month. Set force=true to re-import months that already have data.
//	@Tags			ingest,internal
//	@Accept			json
//	@Produce		json
//	@Param			body	body		model.WeatherIngestRequest	true	"Ingest range and optional stations"
//	@Success		201		{object}	WeatherIngestResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		409		{object}	WeatherIngestConflictResponse
//	@Failure		500		{object}	ErrorResponse
//	@Router			/api/v1/ingest/weather [post]
func (h *WeatherIngestHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.WeatherIngestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: errInvalidJSONBody})
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

	stations := append([]string(nil), req.Stations...)

	var unmatched []string

	if len(stations) == 0 {
		resolved, unresolved, resolveErr := h.resolveStations(ctx)
		if resolveErr != nil {
			h.writeResolveError(w, resolveErr)
			return
		}

		stations = resolved
		unmatched = unresolved
	}

	active, err := h.store.ActiveWeatherIngestMonths(ctx, months)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCheckActiveIngest})
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
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCheckExistingWeather})
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
		job, err := h.store.CreateWeatherIngestJob(ctx, ym.Year, ym.Month, stations)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCreateIngestJob})
			return
		}

		jobs = append(jobs, WeatherIngestJobResponse{
			ID:       job.ID,
			Year:     ym.Year,
			Month:    ym.Month,
			Stations: append([]string(nil), stations...),
			Status:   job.Status,
		})
	}

	writeJSON(w, http.StatusCreated, WeatherIngestResponse{
		Jobs:               jobs,
		MonthsRequested:    len(months),
		UnresolvedAirports: unmatched,
	})
}

func (h *WeatherIngestHandler) resolveStations(ctx context.Context) ([]string, []string, error) {
	if h.resolver == nil {
		return nil, nil, errors.New("stations required when station resolver is not configured")
	}

	return h.resolver.Resolve(ctx)
}

func (h *WeatherIngestHandler) writeResolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, iem.ErrNoFlightAirports), errors.Is(err, iem.ErrNoMatchedStations):
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	default:
		h.logger.Error("resolve weather stations", "error", err)
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to resolve weather stations"})
	}
}
