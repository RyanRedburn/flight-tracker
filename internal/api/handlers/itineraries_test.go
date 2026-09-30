package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestItineraryOutlookOKAndTight(t *testing.T) {
	window := 45
	tests := []struct {
		name        string
		secondDep   string
		wantStatus  string
		wantLayover int
		wantSlack   int
	}{
		{name: "ok", secondDep: "1100", wantStatus: model.ConnectionOK, wantLayover: 115, wantSlack: 55},
		{name: "tight", secondDep: "0940", wantStatus: model.ConnectionTight, wantLayover: 35, wantSlack: -25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				mu      sync.Mutex
				filters []store.RouteOutlookFilter
			)

			h := NewItinerariesHandler(&storetest.Stub{
				RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
					mu.Lock()

					filters = append(filters, filter)
					mu.Unlock()

					return outlookWithMinutes(60, 70, 80, 90), nil
				},
				ListAirportCountriesByIATAFn: func(context.Context, []string) (map[string]string, error) {
					return map[string]string{testOriginBOS: "US", testOriginORD: "US", testDestLAX: "US"}, nil
				},
			})

			rec := postItinerary(t, h, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
				{Origin: "bos", Dest: "ord", Carrier: "ua", Date: testDateTue, DepTime: "700", ArrTime: "905"},
				{Origin: testOriginORD, Dest: testDestLAX, Carrier: "UA", Date: testDateTue, DepTime: tt.secondDep, ArrTime: "1400", DepTimeWindowMinutes: &window},
			}})

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}

			var out model.ItineraryOutlookResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if len(out.Legs) != 2 || len(out.Connections) != 1 {
				t.Fatalf("legs=%d connections=%d", len(out.Legs), len(out.Connections))
			}

			if out.Legs[0].Origin != testOriginBOS || out.Legs[0].DayOfWeek != 2 || out.Legs[0].DepTime != "0700" || out.Legs[0].ArrTime != testArr0905 {
				t.Fatalf("first leg = %+v", out.Legs[0])
			}

			if out.Legs[0].DepTimeWindowMinutes != store.DefaultDepTimeWindowMinutes || out.Legs[1].DepTimeWindowMinutes != 45 {
				t.Fatalf("windows = %d %d", out.Legs[0].DepTimeWindowMinutes, out.Legs[1].DepTimeWindowMinutes)
			}

			if out.Legs[0].Error != nil || out.Legs[0].Outlook == nil || out.Legs[1].Error != nil {
				t.Fatalf("leg errors = %v %v", out.Legs[0].Error, out.Legs[1].Error)
			}

			conn := out.Connections[0]
			if conn.AfterLeg != 0 || conn.Airport != testOriginORD || conn.Status != tt.wantStatus {
				t.Fatalf("connection = %+v", conn)
			}

			if conn.ConnectionType == nil || *conn.ConnectionType != model.ConnectionDomesticToDomestic {
				t.Fatalf("type = %v", conn.ConnectionType)
			}

			assertMinutes(t, "layover", conn.LayoverMinutes, tt.wantLayover)
			assertMinutes(t, "recommended", conn.RecommendedConnectionMinutes, 60)
			assertMinutes(t, "slack", conn.SlackMinutes, tt.wantSlack)

			if len(filters) != 2 {
				t.Fatalf("outlook calls = %d", len(filters))
			}

			for _, filter := range filters {
				if filter.DayOfWeek != 2 || filter.Carrier != "UA" {
					t.Fatalf("filter = %+v", filter)
				}
			}
		})
	}
}

func TestItineraryOutlookOvernight(t *testing.T) {
	h := itineraryHandler(outlookWithMinutes(45, 60, 120, 180), map[string]string{
		testOriginBOS: "US",
		testOriginORD: "US",
		testDestLAX:   "US",
	}, nil)

	rec := postItinerary(t, h, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
		{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "1800", ArrTime: "2200"},
		{Origin: testOriginORD, Dest: testDestLAX, Carrier: "UA", Date: testDateWed, DepTime: "0700", ArrTime: "1000"},
	}})
	out := decodeItinerary(t, rec)
	conn := out.Connections[0]

	if conn.Status != model.ConnectionOK {
		t.Fatalf("status = %s, body = %s", conn.Status, rec.Body.String())
	}

	assertMinutes(t, "layover", conn.LayoverMinutes, 540)

	if out.Legs[1].DayOfWeek != 3 {
		t.Fatalf("outbound weekday = %d, want 3", out.Legs[1].DayOfWeek)
	}
}

func TestItineraryOutlookConnectionTypes(t *testing.T) {
	h := itineraryHandler(outlookWithMinutes(30, 40, 50, 60), map[string]string{
		testOriginBOS:  "US",
		testOriginORD:  "US",
		testAirportLHR: "GB",
		testAirportCDG: "FR",
		testAirportNCE: "FR",
	}, nil)

	rec := postItinerary(t, h, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
		{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "0700", ArrTime: "0900"},
		{Origin: testOriginORD, Dest: testAirportLHR, Carrier: "UA", Date: testDateTue, DepTime: "1200", ArrTime: "2200"},
		{Origin: testAirportLHR, Dest: testAirportCDG, Carrier: "UA", Date: testDateWed, DepTime: "1000", ArrTime: "1200"},
		{Origin: testAirportCDG, Dest: testAirportNCE, Carrier: "UA", Date: testDateWed, DepTime: "1400", ArrTime: "1530"},
	}})
	out := decodeItinerary(t, rec)

	if len(out.Connections) != 3 {
		t.Fatalf("connections = %d", len(out.Connections))
	}

	want := []struct {
		airport string
		typ     string
		rcm     int
	}{
		{testOriginORD, model.ConnectionDomesticToInternational, 40},
		{testAirportLHR, model.ConnectionInternationalToInternational, 60},
		{testAirportCDG, model.ConnectionInternationalToDomestic, 50},
	}

	for i, tt := range want {
		conn := out.Connections[i]
		if conn.AfterLeg != i || conn.Airport != tt.airport || conn.Status != model.ConnectionOK {
			t.Fatalf("connection %d = %+v", i, conn)
		}

		if conn.ConnectionType == nil || *conn.ConnectionType != tt.typ {
			t.Fatalf("connection %d type = %v, want %s", i, conn.ConnectionType, tt.typ)
		}

		assertMinutes(t, "recommended", conn.RecommendedConnectionMinutes, tt.rcm)
	}
}

func TestItineraryOutlookMismatch(t *testing.T) {
	h := itineraryHandler(outlookWithMinutes(45, 60, 120, 180), map[string]string{
		testOriginBOS: "US",
		testOriginORD: "US",
		"DEN":         "US",
		testDestLAX:   "US",
	}, nil)

	rec := postItinerary(t, h, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
		{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "0700", ArrTime: "0900"},
		{Origin: "DEN", Dest: testDestLAX, Carrier: "UA", Date: testDateTue, DepTime: "1200", ArrTime: "1500"},
	}})
	out := decodeItinerary(t, rec)
	conn := out.Connections[0]

	if conn.Status != model.ConnectionUnknown || conn.ConnectionType != nil || conn.RecommendedConnectionMinutes != nil || conn.SlackMinutes != nil {
		t.Fatalf("connection = %+v", conn)
	}

	assertMinutes(t, "layover", conn.LayoverMinutes, 180)

	if conn.Airport != testOriginORD {
		t.Fatalf("airport = %s", conn.Airport)
	}
}

func TestItineraryOutlookMissingLeg(t *testing.T) {
	h := NewItinerariesHandler(&storetest.Stub{
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			if filter.Origin == testOriginBOS {
				return nil, store.ErrNotFound
			}

			out := outlookWithMinutes(45, 60, 120, 180)
			out.Origin = filter.Origin
			out.Dest = filter.Dest

			return out, nil
		},
		ListAirportCountriesByIATAFn: func(context.Context, []string) (map[string]string, error) {
			return map[string]string{testOriginBOS: "US", testOriginORD: "US", testDestLAX: "US"}, nil
		},
	})

	rec := postItinerary(t, h, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
		{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "0700", ArrTime: testArr0905},
		{Origin: testOriginORD, Dest: testDestLAX, Carrier: "UA", Date: testDateTue, DepTime: "1100", ArrTime: "1400"},
	}})
	out := decodeItinerary(t, rec)

	if out.Legs[0].Outlook != nil || out.Legs[0].Error == nil || *out.Legs[0].Error != errRouteOutlookNotFound {
		t.Fatalf("first leg = %+v", out.Legs[0])
	}

	if out.Legs[1].Outlook == nil || out.Legs[1].Error != nil {
		t.Fatalf("second leg = %+v", out.Legs[1])
	}

	conn := out.Connections[0]
	if conn.Status != model.ConnectionUnknown || conn.RecommendedConnectionMinutes != nil || conn.SlackMinutes != nil {
		t.Fatalf("connection = %+v", conn)
	}

	if conn.ConnectionType == nil || *conn.ConnectionType != model.ConnectionDomesticToDomestic {
		t.Fatalf("type = %v", conn.ConnectionType)
	}

	assertNullJSON(t, rec.Body.Bytes(), "legs", 0, "outlook")
	assertNullJSON(t, rec.Body.Bytes(), "legs", 1, "error")
}

func TestItineraryOutlookBadRequest(t *testing.T) {
	h := NewItinerariesHandler(&storetest.Stub{})

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid json", body: "{"},
		{name: "one leg", body: `{"legs":[{"origin":"BOS","dest":"ORD","carrier":"UA","date":"2026-10-06","dep_time":"0700","arr_time":"0905"}]}`},
		{name: "missing time", body: `{"legs":[{"origin":"BOS","dest":"ORD","carrier":"UA","date":"2026-10-06","dep_time":"0700"},{"origin":"ORD","dest":"LAX","carrier":"UA","date":"2026-10-06","dep_time":"1100","arr_time":"1400"}]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/itineraries/outlook", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			h.ItineraryOutlook(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestItineraryOutlookStoreError(t *testing.T) {
	tests := []struct {
		name    string
		outlook error
		country error
	}{
		{name: "outlook", outlook: errors.New("db down")},
		{name: "countries", country: errors.New("db down")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := itineraryHandler(outlookWithMinutes(45, 60, 120, 180), map[string]string{testOriginBOS: "US"}, tt.country)
			if tt.outlook != nil {
				h = NewItinerariesHandler(&storetest.Stub{
					RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
						return nil, tt.outlook
					},
					ListAirportCountriesByIATAFn: func(context.Context, []string) (map[string]string, error) {
						return map[string]string{}, nil
					},
				})
			}

			rec := postItinerary(t, h, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
				{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "0700", ArrTime: testArr0905},
				{Origin: testOriginORD, Dest: testDestLAX, Carrier: "UA", Date: testDateTue, DepTime: "1100", ArrTime: "1400"},
			}})

			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}

			var body ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if body.Error != errFailedItineraryOutlook {
				t.Fatalf("error = %q", body.Error)
			}
		})
	}
}

func itineraryHandler(outlook *model.RouteOutlook, countries map[string]string, countryErr error) *ItinerariesHandler {
	return NewItinerariesHandler(&storetest.Stub{
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return outlook, nil
		},
		ListAirportCountriesByIATAFn: func(context.Context, []string) (map[string]string, error) {
			if countryErr != nil {
				return nil, countryErr
			}

			return countries, nil
		},
	})
}

func postItinerary(t *testing.T, h *ItinerariesHandler, body model.ItineraryOutlookRequest) *httptest.ResponseRecorder {
	t.Helper()

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/itineraries/outlook", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	h.ItineraryOutlook(rec, req)

	return rec
}

func decodeItinerary(t *testing.T, rec *httptest.ResponseRecorder) model.ItineraryOutlookResponse {
	t.Helper()

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var out model.ItineraryOutlookResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}

	return out
}

func outlookWithMinutes(dd, di, id, ii int) *model.RouteOutlook {
	return &model.RouteOutlook{
		RecommendedConnectionMinutes: model.RecommendedConnectionMinutes{
			DomesticToDomestic:           &dd,
			DomesticToInternational:      &di,
			InternationalToDomestic:      &id,
			InternationalToInternational: &ii,
		},
	}
}

func assertMinutes(t *testing.T, name string, got *int, want int) {
	t.Helper()

	if got == nil || *got != want {
		t.Errorf("%s = %v, want %d", name, got, want)
	}
}

func assertNullJSON(t *testing.T, body []byte, arrayKey string, index int, field string) {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw[arrayKey], &items); err != nil {
		t.Fatalf("unmarshal %s: %v", arrayKey, err)
	}

	value, ok := items[index][field]
	if !ok {
		t.Fatalf("missing %s[%d].%s", arrayKey, index, field)
	}

	if string(value) != "null" {
		t.Fatalf("%s[%d].%s = %s, want null", arrayKey, index, field, value)
	}
}
