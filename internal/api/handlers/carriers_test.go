package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestCarriersStats(t *testing.T) {
	h := NewCarriersHandler(&storetest.Stub{
		CarrierStatsFn: func(_ context.Context, filter store.CarrierStatsFilter) (*model.CarrierStats, error) {
			if filter.Carrier != "UA" || filter.State != "IL" {
				t.Errorf("filter = %+v", filter)
			}

			return &model.CarrierStats{
				Carrier:       "UA",
				Flights:       10,
				OnTime:        7,
				BestRoutes:    []model.CarrierRouteStat{},
				WorstRoutes:   []model.CarrierRouteStat{},
				BestAirports:  []model.CarrierAirportStat{},
				WorstAirports: []model.CarrierAirportStat{},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/carriers/stats?carrier=ua&state=il", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var stats model.CarrierStats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if stats.Flights != 10 || stats.OnTime != 7 {
		t.Fatalf("stats unexpected: %+v", stats)
	}
}

func TestCarriersStatsEmpty(t *testing.T) {
	h := NewCarriersHandler(&storetest.Stub{
		CarrierStatsFn: func(context.Context, store.CarrierStatsFilter) (*model.CarrierStats, error) {
			return &model.CarrierStats{
				Carrier:       "UA",
				BestRoutes:    []model.CarrierRouteStat{},
				WorstRoutes:   []model.CarrierRouteStat{},
				BestAirports:  []model.CarrierAirportStat{},
				WorstAirports: []model.CarrierAirportStat{},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/carriers/stats?carrier=UA", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var stats model.CarrierStats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if stats.Flights != 0 || stats.BestRoutes == nil {
		t.Fatalf("empty stats = %+v", stats)
	}
}

func TestCarriersStatsBadRequest(t *testing.T) {
	h := NewCarriersHandler(&storetest.Stub{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/carriers/stats", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCarriersStatsStoreError(t *testing.T) {
	h := NewCarriersHandler(&storetest.Stub{
		CarrierStatsFn: func(context.Context, store.CarrierStatsFilter) (*model.CarrierStats, error) {
			return nil, context.DeadlineExceeded
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/carriers/stats?carrier=UA", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
