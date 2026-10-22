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
		{name: "loose", secondDep: "1100", wantStatus: model.ConnectionLoose, wantLayover: 115, wantSlack: 55},
		{name: "ok", secondDep: "1015", wantStatus: model.ConnectionOK, wantLayover: 70, wantSlack: 10},
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

					out := outlookWithMinutes(60, 70, 80, 90)
					out.Filters = outlookFilters(filter)

					return out, nil
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

			if out.Legs[0].Index != 0 || out.Legs[0].Date != testDateTue {
				t.Fatalf("first leg = %+v", out.Legs[0])
			}

			wantRisk := tt.wantStatus == model.ConnectionTight

			riskJSON := `"connection_risk":false`
			if wantRisk {
				riskJSON = `"connection_risk":true`
			}

			if out.Summary.Origin != testOriginBOS || out.Summary.Destination != testDestLAX || out.Summary.Confidence != model.ConfidenceHigh || out.Summary.ConnectionRisk != wantRisk || !bytes.Contains(rec.Body.Bytes(), []byte(riskJSON)) {
				t.Fatalf("summary = %+v body = %s", out.Summary, rec.Body.Bytes())
			}

			assertNestedOutlookResults(t, rec.Body.Bytes(), 0)

			if out.Legs[0].Outlook == nil || out.Legs[0].Outlook.Filters.DepTimeWindowMinutes != store.DefaultDepTimeWindowMinutes || out.Legs[1].Outlook == nil || out.Legs[1].Outlook.Filters.DepTimeWindowMinutes != 45 {
				t.Fatalf("windows = %+v %+v", out.Legs[0].Outlook, out.Legs[1].Outlook)
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
			assertMinutes(t, "recommended", conn.RecommendedMinutes, 60)
			assertMinutes(t, "loose", conn.LooseMinutes, 90)
			assertMinutes(t, "slack", conn.SlackMinutes, tt.wantSlack)

			if conn.Confidence != model.ConfidenceHigh {
				t.Errorf("confidence = %q, want high", conn.Confidence)
			}

			if conn.Reason != model.ConnectionReasonEvaluated || conn.Overnight || conn.MCTSource == nil || *conn.MCTSource != model.MCTSourceAirport {
				t.Errorf("reason = %q overnight %v mct_source %v", conn.Reason, conn.Overnight, conn.MCTSource)
			}

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

func TestItineraryOutlookWeatherDoesNotChangeConnection(t *testing.T) {
	weather := store.AssembleRouteOutlookWeather([]store.WeatherCategoryCount{
		{Side: store.WeatherSideOrigin, Category: model.WeatherCategoryVFRFair, OnTimeCount: 40, Flights: 50},
		{Side: store.WeatherSideOrigin, Category: model.WeatherCategoryIFR, OnTimeCount: 10, Flights: 40},
		{Side: store.WeatherSideDest, Category: model.WeatherCategoryVFRFair, OnTimeCount: 1, Flights: 2},
		{Side: store.WeatherSideDest, Category: model.WeatherCategoryMVFR, OnTimeCount: 1, Flights: 2},
	})
	outlook := outlookWithMinutes(60, 70, 80, 90)
	outlook.Weather = weather

	h := itineraryHandler(outlook, map[string]string{
		testOriginBOS: "US",
		testOriginORD: "US",
		testDestLAX:   "US",
	}, nil)

	rec := postItinerary(t, h, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
		{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "0700", ArrTime: testArr0905},
		{Origin: testOriginORD, Dest: testDestLAX, Carrier: "UA", Date: testDateTue, DepTime: "1100", ArrTime: "1400"},
	}})
	out := decodeItinerary(t, rec)

	if out.Legs[0].Outlook == nil || out.Legs[1].Outlook == nil {
		t.Fatal("expected both leg outlooks")
	}

	origin := out.Legs[0].Outlook.Weather.Origin
	if origin.Confidence != model.ConfidenceMedium || origin.FairOnTimeRate == nil || *origin.FairOnTimeRate != 0.8 || origin.AdverseOnTimeRate == nil || *origin.AdverseOnTimeRate != 0.25 || origin.AdverseDelta == nil || *origin.AdverseDelta != -0.55 || origin.AdverseFlights != 40 {
		t.Fatalf("leg weather origin = %+v", origin)
	}

	if out.Legs[0].Outlook.Weather.Dest.Confidence != model.ConfidenceLow {
		t.Fatalf("leg weather dest confidence = %q", out.Legs[0].Outlook.Weather.Dest.Confidence)
	}

	if out.Legs[0].Outlook.Analysis.Confidence != model.ConfidenceHigh {
		t.Fatalf("outlook confidence = %q", out.Legs[0].Outlook.Analysis.Confidence)
	}

	conn := out.Connections[0]
	if conn.Status != model.ConnectionLoose || conn.Confidence != model.ConfidenceHigh || conn.Reason != model.ConnectionReasonEvaluated {
		t.Fatalf("connection = %+v", conn)
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

	if conn.Status != model.ConnectionLoose || conn.Reason != model.ConnectionReasonOvernight || !conn.Overnight {
		t.Fatalf("status = %s reason = %s overnight %v, body = %s", conn.Status, conn.Reason, conn.Overnight, rec.Body.String())
	}

	if conn.MCTSource == nil || *conn.MCTSource != model.MCTSourceAirport {
		t.Fatalf("mct_source = %v, want airport", conn.MCTSource)
	}

	assertMinutes(t, "layover", conn.LayoverMinutes, 540)

	if out.Legs[1].Outlook == nil || out.Legs[1].Outlook.Filters.DayOfWeek != 3 || out.Legs[1].Date != testDateWed {
		t.Fatalf("outbound leg = %+v", out.Legs[1])
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
		{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "0700", ArrTime: testArr0900},
		{Origin: testOriginORD, Dest: testAirportLHR, Carrier: "UA", Date: testDateTue, DepTime: "1200", ArrTime: "2200"},
		{Origin: testAirportLHR, Dest: testAirportCDG, Carrier: "UA", Date: testDateWed, DepTime: "1000", ArrTime: "1200"},
		{Origin: testAirportCDG, Dest: testAirportNCE, Carrier: "UA", Date: testDateWed, DepTime: "1400", ArrTime: "1530"},
	}})
	out := decodeItinerary(t, rec)

	if len(out.Connections) != 3 {
		t.Fatalf("connections = %d", len(out.Connections))
	}

	want := []struct {
		airport   string
		typ       string
		rcm       int
		reason    string
		overnight bool
	}{
		{testOriginORD, model.ConnectionDomesticToInternational, 40, model.ConnectionReasonEvaluated, false},
		{testAirportLHR, model.ConnectionInternationalToInternational, 60, model.ConnectionReasonOvernight, true},
		{testAirportCDG, model.ConnectionInternationalToDomestic, 50, model.ConnectionReasonEvaluated, false},
	}

	for i, tt := range want {
		conn := out.Connections[i]
		if conn.AfterLeg != i || conn.Airport != tt.airport || conn.Status != model.ConnectionLoose || conn.Reason != tt.reason || conn.Overnight != tt.overnight {
			t.Fatalf("connection %d = %+v", i, conn)
		}

		if conn.MCTSource == nil || *conn.MCTSource != model.MCTSourceAirport {
			t.Fatalf("connection %d mct_source = %v, want airport", i, conn.MCTSource)
		}

		if conn.ConnectionType == nil || *conn.ConnectionType != tt.typ {
			t.Fatalf("connection %d type = %v, want %s", i, conn.ConnectionType, tt.typ)
		}

		assertMinutes(t, "recommended", conn.RecommendedMinutes, tt.rcm)
		assertMinutes(t, "loose", conn.LooseMinutes, tt.rcm+model.ConnectionLooseSlackMinutes)
	}
}

func TestItineraryOutlookMismatch(t *testing.T) {
	h := itineraryHandler(outlookWithMinutes(45, 60, 120, 180), map[string]string{
		testOriginBOS:  "US",
		testOriginORD:  "US",
		testAirportDEN: "US",
		testDestLAX:    "US",
	}, nil)

	rec := postItinerary(t, h, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
		{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "0700", ArrTime: testArr0900},
		{Origin: testAirportDEN, Dest: testDestLAX, Carrier: "UA", Date: testDateTue, DepTime: "1200", ArrTime: "1500"},
	}})
	out := decodeItinerary(t, rec)
	conn := out.Connections[0]

	if conn.Status != model.ConnectionUnknown || conn.Confidence != model.ConfidenceUnknown || conn.Reason != model.ConnectionReasonMultiAirport || conn.MCTSource != nil || conn.ConnectionType != nil || conn.RecommendedMinutes != nil || conn.LooseMinutes != nil || conn.SlackMinutes != nil {
		t.Fatalf("connection = %+v", conn)
	}

	if out.Legs[0].Outlook == nil || out.Legs[0].Outlook.Analysis.Confidence != model.ConfidenceHigh {
		t.Fatalf("inbound outlook confidence = %+v", out.Legs[0].Outlook)
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
			out.Filters.Origin = filter.Origin
			out.Filters.Dest = filter.Dest

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
	if conn.Status != model.ConnectionUnknown || conn.Confidence != model.ConfidenceUnknown || conn.Reason != model.ConnectionReasonMissingOutlook || conn.MCTSource != nil || conn.RecommendedMinutes != nil || conn.LooseMinutes != nil || conn.SlackMinutes != nil {
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
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			if outlook == nil {
				return nil, nil
			}

			out := *outlook
			out.Filters = outlookFilters(filter)

			return &out, nil
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

func TestItineraryOutlookSparseSamples(t *testing.T) {
	h := NewItinerariesHandler(&storetest.Stub{
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			switch filter.Origin {
			case testOriginBOS:
				return nil, store.ErrNotFound
			case testOriginORD:
				zero := 0.0
				out := &model.RouteOutlook{
					Filters: model.Filters{Origin: filter.Origin, Dest: filter.Dest, Carrier: filter.Carrier},
					OutlookResult: model.OutlookResult{
						Analysis:          model.Analysis{SampleSize: 0},
						OnTimeProbability: &zero,
					},
				}
				out.ApplyOutlookSample()

				return out, nil
			case testDestLAX:
				out := &model.RouteOutlook{
					Filters: model.Filters{Origin: filter.Origin, Dest: filter.Dest, Carrier: filter.Carrier},
					OutlookResult: model.OutlookResult{
						Analysis:          model.Analysis{SampleSize: 4},
						OnTimeProbability: floatPtr(0),
					},
				}
				out.ApplyOutlookSample()

				return out, nil
			default:
				out := outlookWithMinutes(45, 60, 120, 180)
				out.Filters.Origin = filter.Origin
				out.Filters.Dest = filter.Dest

				return out, nil
			}
		},
		ListAirportCountriesByIATAFn: func(context.Context, []string) (map[string]string, error) {
			return map[string]string{
				testOriginBOS:  "US",
				testOriginORD:  "US",
				testDestLAX:    "US",
				testAirportDEN: "US",
			}, nil
		},
	})

	rec := postItinerary(t, h, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
		{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "0700", ArrTime: testArr0905},
		{Origin: testOriginORD, Dest: testDestLAX, Carrier: "UA", Date: testDateTue, DepTime: "1100", ArrTime: "1400"},
		{Origin: testDestLAX, Dest: testAirportDEN, Carrier: "UA", Date: testDateTue, DepTime: "1500", ArrTime: "1700"},
		{Origin: testAirportDEN, Dest: testOriginBOS, Carrier: "UA", Date: testDateTue, DepTime: "1800", ArrTime: "2100"},
	}})
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	out := decodeItinerary(t, rec)

	if out.Legs[0].Outlook != nil || out.Legs[0].Error == nil || *out.Legs[0].Error != errRouteOutlookNotFound {
		t.Fatalf("never-seen leg = %+v", out.Legs[0])
	}

	emptyLeg := out.Legs[1]
	if emptyLeg.Error != nil || emptyLeg.Outlook == nil {
		t.Fatalf("empty leg = %+v", emptyLeg)
	}

	if emptyLeg.Outlook.Analysis.SampleReason != model.SampleReasonEmptySample || emptyLeg.Outlook.Analysis.Confidence != model.ConfidenceUnknown {
		t.Fatalf("empty outlook = %+v", emptyLeg.Outlook)
	}

	if outlookJSONField(t, rec.Body.Bytes(), 1, "on_time_probability") != jsonNull {
		t.Fatalf("empty on_time_probability = %s", outlookJSONField(t, rec.Body.Bytes(), 1, "on_time_probability"))
	}

	thinLeg := out.Legs[2]
	if thinLeg.Error != nil || thinLeg.Outlook == nil {
		t.Fatalf("thin leg = %+v", thinLeg)
	}

	if thinLeg.Outlook.Analysis.SampleReason != model.SampleReasonInsufficientSample || thinLeg.Outlook.Analysis.Confidence != model.ConfidenceLow {
		t.Fatalf("thin outlook = %+v", thinLeg.Outlook)
	}

	if outlookJSONField(t, rec.Body.Bytes(), 2, "on_time_probability") != "0" {
		t.Fatalf("thin on_time_probability = %s, want 0", outlookJSONField(t, rec.Body.Bytes(), 2, "on_time_probability"))
	}

	if out.Legs[3].Outlook == nil || out.Legs[3].Outlook.Analysis.SampleReason != model.SampleReasonSufficient || out.Legs[3].Outlook.Analysis.Confidence != model.ConfidenceHigh {
		t.Fatalf("solid leg = %+v", out.Legs[3])
	}

	if len(out.Connections) != 3 {
		t.Fatalf("connections = %d", len(out.Connections))
	}

	if out.Summary.Origin != testOriginBOS || out.Summary.Destination != testOriginBOS || out.Summary.Confidence != model.ConfidenceUnknown || !out.Summary.ConnectionRisk {
		t.Fatalf("summary = %+v", out.Summary)
	}

	neverSeen := out.Connections[0]
	if neverSeen.Status != model.ConnectionUnknown || neverSeen.Confidence != model.ConfidenceUnknown || neverSeen.Reason != model.ConnectionReasonMissingOutlook || neverSeen.MCTSource != nil || neverSeen.RecommendedMinutes != nil {
		t.Fatalf("never-seen connection = %+v", neverSeen)
	}

	emptyConn := out.Connections[1]
	if emptyConn.Status != model.ConnectionUnknown || emptyConn.Confidence != model.ConfidenceUnknown || emptyConn.Reason != model.SampleReasonEmptySample || emptyConn.MCTSource != nil || emptyConn.RecommendedMinutes != nil {
		t.Fatalf("empty-sample connection = %+v", emptyConn)
	}

	if emptyConn.ConnectionType == nil || *emptyConn.ConnectionType != model.ConnectionDomesticToDomestic {
		t.Fatalf("empty-sample type = %v", emptyConn.ConnectionType)
	}

	thinConn := out.Connections[2]
	if thinConn.Status != model.ConnectionUnknown || thinConn.Confidence != model.ConfidenceLow || thinConn.Reason != model.SampleReasonInsufficientSample || thinConn.MCTSource != nil || thinConn.RecommendedMinutes != nil || thinConn.LooseMinutes != nil {
		t.Fatalf("thin connection = %+v", thinConn)
	}
}

func assertNestedOutlookResults(t *testing.T, body []byte, leg int) {
	t.Helper()

	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, field := range []string{"filters", "analysis"} {
		if _, ok := root[field]; ok {
			t.Fatalf("itinerary repeated %s", field)
		}
	}

	if _, ok := root["summary"]; !ok {
		t.Fatal("itinerary missing summary")
	}

	var legs []map[string]json.RawMessage
	if err := json.Unmarshal(root["legs"], &legs); err != nil {
		t.Fatalf("unmarshal legs: %v", err)
	}

	for _, field := range []string{"origin", "dest", "carrier", "dep_time", "arr_time", "day_of_week", "dep_time_window_minutes"} {
		if _, ok := legs[leg][field]; ok {
			t.Fatalf("leg repeated %s", field)
		}
	}

	outlook := nestedOutlookObject(t, body, leg)

	filtersRaw, ok := outlook["filters"]
	if !ok {
		t.Fatal("nested outlook missing filters")
	}

	var filters model.Filters
	if err := json.Unmarshal(filtersRaw, &filters); err != nil {
		t.Fatalf("decode filters: %v", err)
	}

	if filters.Origin == "" || filters.Dest == "" || filters.Carrier == "" || filters.DepTime == "" || filters.DayOfWeek == 0 || filters.DepTimeWindowMinutes == 0 {
		t.Fatalf("outlook filters = %+v", filters)
	}

	if _, ok := outlook["analysis"]; !ok {
		t.Fatal("nested outlook missing analysis")
	}
}

func outlookFilters(filter store.RouteOutlookFilter) model.Filters {
	return model.Filters{
		Origin:               filter.Origin,
		Dest:                 filter.Dest,
		Carrier:              filter.Carrier,
		DayOfWeek:            filter.DayOfWeek,
		DepTime:              filter.DepTime,
		DepTimeWindowMinutes: filter.DepTimeWindowMinutes,
	}
}

func nestedOutlookObject(t *testing.T, body []byte, leg int) map[string]json.RawMessage {
	t.Helper()

	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	var legs []map[string]json.RawMessage
	if err := json.Unmarshal(root["legs"], &legs); err != nil {
		t.Fatalf("unmarshal legs: %v", err)
	}

	var outlook map[string]json.RawMessage
	if err := json.Unmarshal(legs[leg]["outlook"], &outlook); err != nil {
		t.Fatalf("unmarshal outlook: %v", err)
	}

	return outlook
}

func outlookJSONField(t *testing.T, body []byte, leg int, field string) string {
	t.Helper()

	return string(nestedOutlookObject(t, body, leg)[field])
}

func outlookWithMinutes(dd, di, id, ii int) *model.RouteOutlook {
	guidance := model.ConnectionGuidanceFromRecommended(dd, di, id, ii)
	for _, bucket := range []*model.ConnectionMinutes{
		guidance.DomesticToDomestic,
		guidance.DomesticToInternational,
		guidance.InternationalToDomestic,
		guidance.InternationalToInternational,
	} {
		if bucket != nil {
			bucket.MCTSource = model.MCTSourceAirport
		}
	}

	return &model.RouteOutlook{
		OutlookResult: model.OutlookResult{
			Analysis: model.Analysis{
				Start:        "2025-10-06",
				End:          "2026-10-06",
				SampleSize:   40,
				SampleReason: model.SampleReasonSufficient,
				Confidence:   model.ConfidenceHigh,
			},
			Connection: guidance,
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

	if string(value) != jsonNull {
		t.Fatalf("%s[%d].%s = %s, want null", arrayKey, index, field, value)
	}
}
