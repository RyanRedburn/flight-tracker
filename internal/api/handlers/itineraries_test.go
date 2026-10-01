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
			assertMinutes(t, "recommended", conn.RecommendedMinutes, 60)
			assertMinutes(t, "loose", conn.LooseMinutes, 90)
			assertMinutes(t, "slack", conn.SlackMinutes, tt.wantSlack)

			if conn.Confidence != model.ConfidenceHigh {
				t.Errorf("confidence = %q, want high", conn.Confidence)
			}

			if conn.Reason != model.ConnectionReasonEvaluated || conn.Overnight || conn.FloorOnly == nil || *conn.FloorOnly {
				t.Errorf("reason = %q overnight %v floor_only %v", conn.Reason, conn.Overnight, conn.FloorOnly)
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

	if conn.FloorOnly == nil || *conn.FloorOnly {
		t.Fatalf("floor_only = %v, want false", conn.FloorOnly)
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

		if conn.FloorOnly == nil || *conn.FloorOnly {
			t.Fatalf("connection %d floor_only = %v, want false", i, conn.FloorOnly)
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

	if conn.Status != model.ConnectionUnknown || conn.Confidence != model.ConfidenceUnknown || conn.Reason != model.ConnectionReasonMultiAirport || conn.FloorOnly != nil || conn.ConnectionType != nil || conn.RecommendedMinutes != nil || conn.LooseMinutes != nil || conn.SlackMinutes != nil {
		t.Fatalf("connection = %+v", conn)
	}

	if out.Legs[0].Outlook == nil || out.Legs[0].Outlook.Confidence != model.ConfidenceHigh {
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
	if conn.Status != model.ConnectionUnknown || conn.Confidence != model.ConfidenceUnknown || conn.Reason != model.ConnectionReasonMissingOutlook || conn.FloorOnly != nil || conn.RecommendedMinutes != nil || conn.LooseMinutes != nil || conn.SlackMinutes != nil {
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

func TestItineraryOutlookSparseSamples(t *testing.T) {
	h := NewItinerariesHandler(&storetest.Stub{
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			switch filter.Origin {
			case testOriginBOS:
				return nil, store.ErrNotFound
			case testOriginORD:
				zero := 0.0
				out := &model.RouteOutlook{Origin: filter.Origin, Dest: filter.Dest, Carrier: filter.Carrier, SampleSize: 0, OnTimeProbability: &zero}
				out.ApplyOutlookSample(store.MinOutlookSampleSize)

				return out, nil
			case testDestLAX:
				out := &model.RouteOutlook{
					Origin:            filter.Origin,
					Dest:              filter.Dest,
					Carrier:           filter.Carrier,
					SampleSize:        4,
					OnTimeProbability: floatPtr(0),
				}
				out.ApplyOutlookSample(store.MinOutlookSampleSize)

				return out, nil
			default:
				out := outlookWithMinutes(45, 60, 120, 180)
				out.Origin = filter.Origin
				out.Dest = filter.Dest

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

	if emptyLeg.Outlook.SampleReason != model.SampleReasonEmptySample || emptyLeg.Outlook.Confidence != model.ConfidenceUnknown || emptyLeg.Outlook.InsufficientSample {
		t.Fatalf("empty outlook = %+v", emptyLeg.Outlook)
	}

	if outlookJSONField(t, rec.Body.Bytes(), 1, "on_time_probability") != jsonNull {
		t.Fatalf("empty on_time_probability = %s", outlookJSONField(t, rec.Body.Bytes(), 1, "on_time_probability"))
	}

	thinLeg := out.Legs[2]
	if thinLeg.Error != nil || thinLeg.Outlook == nil {
		t.Fatalf("thin leg = %+v", thinLeg)
	}

	if thinLeg.Outlook.SampleReason != model.SampleReasonInsufficientSample || !thinLeg.Outlook.InsufficientSample || thinLeg.Outlook.Confidence != model.ConfidenceLow {
		t.Fatalf("thin outlook = %+v", thinLeg.Outlook)
	}

	if outlookJSONField(t, rec.Body.Bytes(), 2, "on_time_probability") != "0" {
		t.Fatalf("thin on_time_probability = %s, want 0", outlookJSONField(t, rec.Body.Bytes(), 2, "on_time_probability"))
	}

	if out.Legs[3].Outlook == nil || out.Legs[3].Outlook.SampleReason != model.SampleReasonSufficient || out.Legs[3].Outlook.Confidence != model.ConfidenceHigh {
		t.Fatalf("solid leg = %+v", out.Legs[3])
	}

	if len(out.Connections) != 3 {
		t.Fatalf("connections = %d", len(out.Connections))
	}

	neverSeen := out.Connections[0]
	if neverSeen.Status != model.ConnectionUnknown || neverSeen.Confidence != model.ConfidenceUnknown || neverSeen.Reason != model.ConnectionReasonMissingOutlook || neverSeen.FloorOnly != nil || neverSeen.RecommendedMinutes != nil {
		t.Fatalf("never-seen connection = %+v", neverSeen)
	}

	emptyConn := out.Connections[1]
	if emptyConn.Status != model.ConnectionUnknown || emptyConn.Confidence != model.ConfidenceUnknown || emptyConn.Reason != model.SampleReasonEmptySample || emptyConn.FloorOnly != nil || emptyConn.RecommendedMinutes != nil {
		t.Fatalf("empty-sample connection = %+v", emptyConn)
	}

	if emptyConn.ConnectionType == nil || *emptyConn.ConnectionType != model.ConnectionDomesticToDomestic {
		t.Fatalf("empty-sample type = %v", emptyConn.ConnectionType)
	}

	thinConn := out.Connections[2]
	if thinConn.Status != model.ConnectionUnknown || thinConn.Confidence != model.ConfidenceLow || thinConn.Reason != model.SampleReasonInsufficientSample || thinConn.FloorOnly != nil || thinConn.RecommendedMinutes != nil || thinConn.LooseMinutes != nil {
		t.Fatalf("thin connection = %+v", thinConn)
	}
}

func outlookJSONField(t *testing.T, body []byte, leg int, field string) string {
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

	return string(outlook[field])
}

func outlookWithMinutes(dd, di, id, ii int) *model.RouteOutlook {
	return &model.RouteOutlook{
		SampleSize:         40,
		InsufficientSample: false,
		SampleReason:       model.SampleReasonSufficient,
		Confidence:         model.ConfidenceHigh,
		Connection:         model.ConnectionGuidanceFromRecommended(dd, di, id, ii),
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
