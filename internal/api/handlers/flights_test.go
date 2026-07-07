package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/mem"
)

const (
	testFlightDate = "2026-04-24"
	testAirportORD = "ORD"
	testAirportBHM = "BHM"
	testAirportAVP = "AVP"
)

func newTestFlightsHandler(t *testing.T) (*FlightsHandler, *mem.Store) {
	t.Helper()

	store := mem.New()
	store.SetOnTimeFlights([]*model.OnTimeFlight{
		{
			FlightDate:                      testFlightDate,
			Origin:                          testAirportORD,
			Dest:                            testAirportBHM,
			IATA_Code_Marketing_Airline:     "UA",
			Flight_Number_Marketing_Airline: "4547",
			CRSDepTime:                      "1535",
			DepTime:                         "1525",
			DepDelay:                        "-10.00",
		},
		{
			FlightDate:                      testFlightDate,
			Origin:                          testAirportORD,
			Dest:                            testAirportAVP,
			IATA_Code_Marketing_Airline:     "UA",
			Flight_Number_Marketing_Airline: "4546",
			CRSDepTime:                      "1805",
		},
		{
			FlightDate:                      "2026-04-25",
			Origin:                          "LAX",
			Dest:                            "SFO",
			IATA_Code_Marketing_Airline:     "UA",
			Flight_Number_Marketing_Airline: "100",
			CRSDepTime:                      "0900",
		},
	})

	return NewFlightsHandler(store), store
}

func TestFlightsListFilters(t *testing.T) {
	h, _ := newTestFlightsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/flights?flight_date=2026-04-24&origin=ORD&dest=BHM", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var flights []model.OnTimeFlight
	if err := json.NewDecoder(rec.Body).Decode(&flights); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if len(flights) != 1 {
		t.Fatalf("len(flights) = %d, want 1", len(flights))
	}

	if flights[0].Dest != "BHM" {
		t.Errorf("Dest = %q, want BHM", flights[0].Dest)
	}
}

func TestFlightsListReturnsEmptyArray(t *testing.T) {
	h := NewFlightsHandler(mem.New())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/flights", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	if rec.Body.String() != "[]\n" {
		t.Errorf("body = %q, want []", rec.Body.String())
	}
}

func TestFlightsListInvalidLimit(t *testing.T) {
	h, _ := newTestFlightsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/flights?limit=abc", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestFlightsListInvalidOffset(t *testing.T) {
	h, _ := newTestFlightsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/flights?offset=-1", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestFlightsListInvalidOrigin(t *testing.T) {
	h, _ := newTestFlightsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/flights?origin=OR", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestFlightsListInvalidFlightDate(t *testing.T) {
	h, _ := newTestFlightsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/flights?flight_date=not-a-date", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestFlightsListLimitAndOffset(t *testing.T) {
	h, _ := newTestFlightsHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/flights?limit=1&offset=1", nil)
	rec := httptest.NewRecorder()
	h.List(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var flights []model.OnTimeFlight
	if err := json.NewDecoder(rec.Body).Decode(&flights); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if len(flights) != 1 {
		t.Fatalf("len(flights) = %d, want 1", len(flights))
	}

	if flights[0].Origin != testAirportORD || flights[0].Dest != testAirportAVP {
		t.Errorf("flight = %+v, want ORD->AVP", flights[0])
	}
}
