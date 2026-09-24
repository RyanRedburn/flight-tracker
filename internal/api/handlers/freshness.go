package handlers

import (
	"net/http"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

// DataFreshnessResponse lists freshness for each current dataset.
type DataFreshnessResponse struct {
	Datasets []DatasetFreshnessResponse `json:"datasets"`
}

// DatasetFreshnessResponse is one dataset's last successful ingest and covered period.
// Null ingest fields mean that signal does not exist (never completed, or no rows for the period).
type DatasetFreshnessResponse struct {
	ID                     string                   `json:"id"`
	LastSuccessfulIngestAt *string                  `json:"last_successful_ingest_at"`
	LatestPeriod           *FreshnessPeriodResponse `json:"latest_period"`
	LastSuccessfulJobID    *string                  `json:"last_successful_job_id"`
}

// FreshnessPeriodResponse is a covered month or a full-table snapshot.
type FreshnessPeriodResponse struct {
	// Type is "month" or "snapshot".
	Type string `json:"type"`
	// Year is set when Type is "month".
	Year *int `json:"year,omitempty"`
	// Month is set when Type is "month" (1-12).
	Month *int `json:"month,omitempty"`
	// AsOf is the snapshot timestamp of the last completed replace. Omitted when no completed job exists.
	AsOf *string `json:"as_of,omitempty"`
	// RowCount is the replaced table size for a snapshot. For weather_stations this is the station catalog; the snapshot is also present when airport_weather_stations has rows.
	RowCount *int64 `json:"row_count,omitempty"`
}

type FreshnessHandler struct {
	store store.Store
}

func NewFreshnessHandler(s store.Store) *FreshnessHandler {
	return &FreshnessHandler{store: s}
}

// Get returns last successful ingest time and the latest covered period for each dataset.
//
//	@Summary		Dataset freshness
//	@Description	Admin read of how fresh each ingested dataset is. last_successful_ingest_at and last_successful_job_id come from the newest completed job of that type. latest_period is the max year/month present for flight_performance and weather_observations, or a snapshot (as_of plus row_count) for weather_stations, countries, regions, and airports. Timestamps and periods are null when that dataset has never been ingested and, for snapshots, has no rows. weather_stations row_count is the station catalog; that snapshot is also present when airport_weather_stations has rows.
//	@Tags			freshness,internal
//	@Produce		json
//	@Success		200	{object}	DataFreshnessResponse
//	@Failure		401	{object}	ErrorResponse
//	@Failure		403	{object}	ErrorResponse
//	@Failure		429	{object}	ErrorResponse
//	@Failure		500	{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/data-freshness [get]
func (h *FreshnessHandler) Get(w http.ResponseWriter, r *http.Request) {
	freshness, err := h.store.DataFreshness(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to load data freshness"})
		return
	}

	writeJSON(w, http.StatusOK, toDataFreshnessResponse(freshness))
}

func toDataFreshnessResponse(in model.DataFreshness) DataFreshnessResponse {
	out := DataFreshnessResponse{
		Datasets: make([]DatasetFreshnessResponse, 0, len(in.Datasets)),
	}

	for _, ds := range in.Datasets {
		out.Datasets = append(out.Datasets, toDatasetFreshnessResponse(ds))
	}

	return out
}

func toDatasetFreshnessResponse(ds model.DatasetFreshness) DatasetFreshnessResponse {
	resp := DatasetFreshnessResponse{
		ID:                     ds.ID,
		LastSuccessfulIngestAt: formatTimePtr(ds.LastSuccessfulIngestAt),
		LastSuccessfulJobID:    ds.LastSuccessfulJobID,
	}

	if ds.LatestPeriod != nil {
		resp.LatestPeriod = toFreshnessPeriodResponse(*ds.LatestPeriod)
	}

	return resp
}

func toFreshnessPeriodResponse(p model.FreshnessPeriod) *FreshnessPeriodResponse {
	return &FreshnessPeriodResponse{
		Type:     p.Type,
		Year:     p.Year,
		Month:    p.Month,
		AsOf:     formatTimePtr(p.AsOf),
		RowCount: p.RowCount,
	}
}

func formatTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}

	formatted := t.UTC().Format(time.RFC3339)

	return &formatted
}
