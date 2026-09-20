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
	testOriginORD          = "ORD"
	testDestLAX            = "LAX"
	testDestJFK            = "JFK"
	jsonCarrierOnTimeRates = "carrier_on_time_rates"
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
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ord&dest=lax&start_date=2026-04-01&end_date=2026-04-30&carrier=ua", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.Bytes()

	var stats model.RouteStats
	if err := json.Unmarshal(body, &stats); err != nil {
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

	if jsonHasKey(t, body, jsonCarrierOnTimeRates) {
		t.Fatalf("carrier_on_time_rates should be omitted when carrier is set: %s", body)
	}
}

func TestRoutesStatsEmpty(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteStatsFn: func(context.Context, store.RouteStatsFilter) (*model.RouteStats, error) {
			return &model.RouteStats{
				Origin:             testOriginORD,
				Dest:               testDestLAX,
				DiversionAirports:  []model.AirportCount{},
				CarrierOnTimeRates: []model.CarrierOnTimeRate{},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2026-04-01&end_date=2026-04-30", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	body := rec.Body.Bytes()

	var stats model.RouteStats
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if stats.Flights != 0 || stats.DiversionAirports == nil {
		t.Fatalf("empty stats = %+v", stats)
	}

	if stats.CarrierOnTimeRates == nil {
		t.Fatal("carrier_on_time_rates should be present when carrier is omitted")
	}

	if !jsonHasKey(t, body, jsonCarrierOnTimeRates) {
		t.Fatalf("carrier_on_time_rates missing from JSON: %s", body)
	}
}

func TestRoutesStatsCarrierOnTimeRates(t *testing.T) {
	h := NewRoutesHandler(&storetest.Stub{
		RouteStatsFn: func(_ context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error) {
			if filter.Carrier != "" {
				t.Fatalf("carrier = %q, want empty", filter.Carrier)
			}

			return &model.RouteStats{
				Origin:  testOriginORD,
				Dest:    testDestLAX,
				Flights: 10,
				OnTime:  8,
				Delayed: 2,
				CarrierOnTimeRates: []model.CarrierOnTimeRate{
					{Carrier: "UA", Flights: 6, OnTime: 5, Delayed: 1, OnTimeRate: 0.83},
					{Carrier: "AA", Flights: 4, OnTime: 3, Delayed: 1, OnTimeRate: 0.75},
				},
			}, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX&start_date=2026-04-01&end_date=2026-04-30", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var stats model.RouteStats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(stats.CarrierOnTimeRates) != 2 {
		t.Fatalf("carrier_on_time_rates len = %d, want 2", len(stats.CarrierOnTimeRates))
	}

	if stats.CarrierOnTimeRates[0].Carrier != "UA" || stats.CarrierOnTimeRates[0].OnTimeRate != 0.83 {
		t.Errorf("first rate = %+v", stats.CarrierOnTimeRates[0])
	}

	if stats.CarrierOnTimeRates[1].Carrier != "AA" || stats.CarrierOnTimeRates[1].Flights != 4 {
		t.Errorf("second rate = %+v", stats.CarrierOnTimeRates[1])
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

func jsonHasKey(t *testing.T, body []byte, field string) bool {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	_, ok := raw[field]

	return ok
}
