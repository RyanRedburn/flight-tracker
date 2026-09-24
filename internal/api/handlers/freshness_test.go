package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestFreshnessGetEmpty(t *testing.T) {
	h := NewFreshnessHandler(&storetest.Stub{
		DataFreshnessFn: func(context.Context) (model.DataFreshness, error) {
			return store.AssembleDataFreshness(nil), nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-freshness", nil)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Datasets []json.RawMessage `json:"datasets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(body.Datasets) != 6 {
		t.Fatalf("len = %d, want 6", len(body.Datasets))
	}

	for _, raw := range body.Datasets {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			t.Fatalf("decode dataset: %v", err)
		}

		for _, key := range []string{"last_successful_ingest_at", "latest_period", "last_successful_job_id"} {
			if string(fields[key]) != "null" {
				t.Errorf("%s = %s, want null", key, fields[key])
			}
		}
	}
}

func TestFreshnessGetPartial(t *testing.T) {
	ended := time.Date(2026, 9, 18, 14, 22, 0, 123456789, time.UTC)

	h := NewFreshnessHandler(&storetest.Stub{
		DataFreshnessFn: func(context.Context) (model.DataFreshness, error) {
			return store.AssembleDataFreshness(map[string]store.FreshnessInput{
				model.DatasetIDFlightPerformance: {
					Kind:     store.FreshnessKindMonth,
					JobID:    "job-fp",
					EndedAt:  &ended,
					Year:     2026,
					Month:    6,
					HasMonth: true,
				},
				model.DatasetIDCountries: {
					Kind:     store.FreshnessKindSnapshot,
					JobID:    "job-countries",
					EndedAt:  &ended,
					RowCount: 247,
					HasRows:  true,
				},
				model.DatasetIDRegions: {
					Kind:     store.FreshnessKindSnapshot,
					RowCount: 0,
					HasRows:  false,
				},
				model.DatasetIDWeatherStations: {
					Kind:     store.FreshnessKindSnapshot,
					RowCount: 0,
					HasRows:  true,
				},
			}), nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-freshness", nil)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var body DataFreshnessResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	byID := make(map[string]DatasetFreshnessResponse, len(body.Datasets))
	for _, ds := range body.Datasets {
		byID[ds.ID] = ds
	}

	fp := byID[model.DatasetIDFlightPerformance]
	if fp.LastSuccessfulIngestAt == nil || *fp.LastSuccessfulIngestAt != "2026-09-18T14:22:00Z" {
		t.Fatalf("flight ingest at = %v, want 2026-09-18T14:22:00Z", fp.LastSuccessfulIngestAt)
	}

	if fp.LastSuccessfulJobID == nil || *fp.LastSuccessfulJobID != "job-fp" {
		t.Fatalf("flight job = %v", fp.LastSuccessfulJobID)
	}

	if fp.LatestPeriod == nil || fp.LatestPeriod.Type != model.FreshnessPeriodTypeMonth {
		t.Fatalf("flight period = %+v", fp.LatestPeriod)
	}

	if fp.LatestPeriod.Year == nil || *fp.LatestPeriod.Year != 2026 || fp.LatestPeriod.Month == nil || *fp.LatestPeriod.Month != 6 {
		t.Fatalf("flight period = %+v, want 2026-06", fp.LatestPeriod)
	}

	if fp.LatestPeriod.AsOf != nil || fp.LatestPeriod.RowCount != nil {
		t.Fatalf("flight period includes snapshot fields: %+v", fp.LatestPeriod)
	}

	countries := byID[model.DatasetIDCountries]
	if countries.LatestPeriod == nil || countries.LatestPeriod.Type != model.FreshnessPeriodTypeSnapshot {
		t.Fatalf("countries period = %+v", countries.LatestPeriod)
	}

	if countries.LatestPeriod.AsOf == nil || *countries.LatestPeriod.AsOf != "2026-09-18T14:22:00Z" {
		t.Fatalf("countries as_of = %v", countries.LatestPeriod.AsOf)
	}

	if countries.LatestPeriod.RowCount == nil || *countries.LatestPeriod.RowCount != 247 {
		t.Fatalf("countries row_count = %v", countries.LatestPeriod.RowCount)
	}

	regions := byID[model.DatasetIDRegions]
	if regions.LatestPeriod != nil || regions.LastSuccessfulIngestAt != nil || regions.LastSuccessfulJobID != nil {
		t.Fatalf("regions = %+v, want empty", regions)
	}

	weather := byID[model.DatasetIDWeatherObservations]
	if weather.LatestPeriod != nil || weather.LastSuccessfulJobID != nil {
		t.Fatalf("weather = %+v, want empty", weather)
	}

	stations := byID[model.DatasetIDWeatherStations]
	if stations.LatestPeriod == nil || stations.LatestPeriod.RowCount == nil || *stations.LatestPeriod.RowCount != 0 {
		t.Fatalf("stations period = %+v, want row_count 0", stations.LatestPeriod)
	}

	if stations.LatestPeriod.AsOf != nil || stations.LastSuccessfulIngestAt != nil {
		t.Fatalf("stations timestamps = period %+v ingest %v, want absent", stations.LatestPeriod, stations.LastSuccessfulIngestAt)
	}
}

func TestFreshnessGetStoreError(t *testing.T) {
	h := NewFreshnessHandler(&storetest.Stub{
		DataFreshnessFn: func(context.Context) (model.DataFreshness, error) {
			return model.DataFreshness{}, errors.New("db down")
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/data-freshness", nil)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	var body ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Error != "failed to load data freshness" {
		t.Fatalf("error = %q", body.Error)
	}
}
