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

const (
	testOriginORD = "ORD"
	testDestLAX   = "LAX"
)

func TestRoutesStats(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return &model.RouteStats{
				Origin:    testOriginORD,
				Dest:      testDestLAX,
				Flights:   5,
				OnTime:    2,
				Delayed:   1,
				Cancelled: 1,
				Diverted:  1,
				DiversionAirports: []model.AirportCount{
					{Airport: "MDW", Count: 1},
				},
				CancellationCodes: []model.CancellationCodeCount{
					{Code: "A", Count: 1},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ord&dest=lax&start_date=2026-04-01&end_date=2026-04-30&carrier=ua", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var stats model.RouteStats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if stats.Flights != 5 {
		t.Fatalf("flights = %d, want 5", stats.Flights)
	}

	if stats.OnTime != 2 || stats.Delayed != 1 || stats.Cancelled != 1 || stats.Diverted != 1 {
		t.Fatalf("counts unexpected: %+v", stats)
	}

	if len(stats.DiversionAirports) != 1 || stats.DiversionAirports[0].Airport != "MDW" {
		t.Errorf("diversion_airports = %+v", stats.DiversionAirports)
	}

	if len(stats.CancellationCodes) != 1 || stats.CancellationCodes[0].Code != "A" {
		t.Errorf("cancellation_codes = %+v", stats.CancellationCodes)
	}
}

func TestRoutesStatsEmpty(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return &model.RouteStats{
				Origin:            testOriginORD,
				Dest:              testDestLAX,
				DiversionAirports: []model.AirportCount{},
				CancellationCodes: []model.CancellationCodeCount{},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2026-04-01&end_date=2026-04-30", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var stats model.RouteStats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if stats.Flights != 0 || stats.DiversionAirports == nil || stats.CancellationCodes == nil {
		t.Fatalf("empty stats = %+v", stats)
	}
}

func TestRoutesStatsBadRequest(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRoutesOutlook(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return &model.RouteOutlook{
				Origin:             testOriginORD,
				Dest:               testDestLAX,
				Carrier:            "UA",
				DayOfWeek:          3,
				DepTime:            "0700",
				SampleSize:         3,
				InsufficientSample: true,
				AnalysisEnd:        "2026-04-15",
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=3&dep_time=0700", nil)
	rec := httptest.NewRecorder()
	h.Outlook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var out model.RouteOutlook
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if out.SampleSize != 3 {
		t.Fatalf("sample_size = %d, want 3", out.SampleSize)
	}

	if !out.InsufficientSample {
		t.Fatal("expected insufficient_sample")
	}

	if out.AnalysisEnd != "2026-04-15" {
		t.Errorf("analysis_end = %q, want 2026-04-15", out.AnalysisEnd)
	}
}

func TestRoutesOutlookBadRequest(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=3", nil)
	rec := httptest.NewRecorder()
	h.Outlook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
