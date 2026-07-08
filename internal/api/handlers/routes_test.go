package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

func newTestRoutesHandler(t *testing.T) *RoutesHandler {
	t.Helper()

	s := mem.New()
	s.SetOnTimeFlights([]*model.OnTimeFlight{
		{
			FlightDate: testFlightDate20260401, DayOfWeek: "3", Origin: testAirportORD, Dest: testAirportLAX,
			IATA_Code_Marketing_Airline: "UA", Flight_Number_Marketing_Airline: "100",
			CRSDepTime: "0700", ArrDelayMinutes: testFloatNo, DepDelayMinutes: testFloatNo,
			ArrDel15: testFloatNo, Cancelled: testFloatNo, Diverted: testFloatNo,
		},
		{
			FlightDate: "2026-04-08", DayOfWeek: "3", Origin: testAirportORD, Dest: testAirportLAX,
			IATA_Code_Marketing_Airline: "UA", Flight_Number_Marketing_Airline: "100",
			CRSDepTime: "0705", ArrDelayMinutes: "25.00", DepDelayMinutes: "10.00",
			ArrDel15: testFloatYes, Cancelled: testFloatNo, Diverted: testFloatNo,
			CarrierDelay: "25.00",
		},
		{
			FlightDate: "2026-04-15", DayOfWeek: "3", Origin: testAirportORD, Dest: testAirportLAX,
			IATA_Code_Marketing_Airline: "UA", Flight_Number_Marketing_Airline: "100",
			CRSDepTime: "0710", ArrDelayMinutes: testFloatNo, DepDelayMinutes: testFloatNo,
			ArrDel15: testFloatNo, Cancelled: testFloatNo, Diverted: testFloatNo,
		},
		{
			FlightDate: "2026-04-02", DayOfWeek: "4", Origin: testAirportORD, Dest: testAirportLAX,
			IATA_Code_Marketing_Airline: "UA", Flight_Number_Marketing_Airline: "100",
			CRSDepTime: "0700", Cancelled: testFloatYes, CancellationCode: "A", Diverted: testFloatNo,
		},
		{
			FlightDate: "2026-04-03", DayOfWeek: "5", Origin: testAirportORD, Dest: testAirportLAX,
			IATA_Code_Marketing_Airline: "UA", Flight_Number_Marketing_Airline: "100",
			CRSDepTime: "0700", Cancelled: testFloatNo, Diverted: testFloatYes, Div1Airport: "MDW",
		},
	})

	return NewRoutesHandler(s)
}

func TestRoutesStats(t *testing.T) {
	h := newTestRoutesHandler(t)

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
	h := NewRoutesHandler(mem.New())

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
	h := newTestRoutesHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/stats?origin=ORD&dest=LAX", nil)
	rec := httptest.NewRecorder()
	h.Stats(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRoutesOutlook(t *testing.T) {
	h := newTestRoutesHandler(t)

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
	h := newTestRoutesHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/outlook?origin=ORD&dest=LAX&carrier=UA&day_of_week=3", nil)
	rec := httptest.NewRecorder()
	h.Outlook(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
