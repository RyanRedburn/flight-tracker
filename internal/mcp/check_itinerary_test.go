package mcp

import (
	"context"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCheckItineraryTightConnection(t *testing.T) {
	window := 30
	stub := itineraryOutlookStub(t)

	res := callTool(t, stub, toolCheckItinerary, map[string]any{
		jsonKeyLegs: []map[string]any{
			{
				jsonKeyOrigin: testCodeBOS, jsonKeyDest: testCodeORD, jsonKeyCarrier: testCarrierUA,
				jsonKeyDate: testDateOct6, jsonKeyDepTime: "700", jsonKeyArrTime: "905",
			},
			{
				jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyCarrier: testCarrierUA,
				jsonKeyDate: testDateOct6, jsonKeyDepTime: "940", jsonKeyArrTime: "1400",
				"dep_time_window_minutes": window,
			},
		},
	})
	if res.IsError {
		t.Fatalf("IsError content=%s", toolText(t, res))
	}

	out := decodeResult[checkItineraryData](t, res)
	assertTextJSONEqualsStructured(t, res)

	if out.Confidence != model.ConfidenceHigh {
		t.Fatalf("confidence = %q", out.Confidence)
	}

	if out.Summary != "Connection at ORD looks tight; allow more time or pick a later outbound." {
		t.Fatalf("summary = %q", out.Summary)
	}
}

func TestCheckItineraryTwoTightConnections(t *testing.T) {
	res := callTool(t, multiLegOutlookStub(""), toolCheckItinerary, map[string]any{
		jsonKeyLegs: threeLegItinerary(),
	})
	if res.IsError {
		t.Fatalf("IsError content=%s", toolText(t, res))
	}

	out := decodeResult[checkItineraryData](t, res)

	want := "Connections at ORD and DFW look tight; allow more time or pick later outbounds."
	if out.Summary != want {
		t.Fatalf("summary = %q", out.Summary)
	}
}

func TestCheckItineraryTightAndUnknownConnections(t *testing.T) {
	res := callTool(t, multiLegOutlookStub(testCodeORD+"-"+testCodeDFW), toolCheckItinerary, map[string]any{
		jsonKeyLegs: threeLegItinerary(),
	})
	if res.IsError {
		t.Fatalf("IsError content=%s", toolText(t, res))
	}

	out := decodeResult[checkItineraryData](t, res)

	want := "Connection at ORD looks tight and risk at DFW is unknown; allow more time or pick a later outbound."
	if out.Summary != want {
		t.Fatalf("summary = %q", out.Summary)
	}
}

func TestCheckItineraryFlightNumberAmbiguous(t *testing.T) {
	stub := &storetest.Stub{
		FlightNumberRoutesFn: func(context.Context, store.FlightNumberRoutesFilter) ([]model.FlightNumberRoute, error) {
			return []model.FlightNumberRoute{
				{Origin: testCodeORD, Dest: testCodeLAX, Flights: 40, UsualDepTime: "0700"},
				{Origin: testCodeORD, Dest: testCodeSFO, Flights: 35, UsualDepTime: "0715"},
			}, nil
		},
	}

	res := callTool(t, stub, toolCheckItinerary, map[string]any{
		jsonKeyLegs: []map[string]any{
			{
				jsonKeyCarrier: testCarrierUA, jsonKeyFlightNumber: "100",
				jsonKeyDate: testDateOct6, jsonKeyDepTime: "700", jsonKeyArrTime: "905",
			},
			{
				jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyCarrier: testCarrierUA,
				jsonKeyDate: testDateOct6, jsonKeyDepTime: "1100", jsonKeyArrTime: "1400",
			},
		},
	})
	if !res.IsError {
		t.Fatal("expected error")
	}

	out := decodeResult[checkItineraryData](t, res)
	if out.Error == nil || out.Error.Code != CodeFlightNumberAmbiguous {
		t.Fatalf("error = %#v", out.Error)
	}

	if len(out.Error.FlightRoutes) != 2 {
		t.Fatalf("flight_routes = %#v", out.Error.FlightRoutes)
	}

	want := "UA 100 has more than one route near 0700. Pick origin and destination."
	if out.Error.Message != want {
		t.Fatalf("message = %q", out.Error.Message)
	}
}

func TestCheckItineraryResolvesFlightNumber(t *testing.T) {
	stub := itineraryOutlookStub(t)
	stub.FlightNumberRoutesFn = func(_ context.Context, filter store.FlightNumberRoutesFilter) ([]model.FlightNumberRoute, error) {
		if filter.Carrier != testCarrierUA || filter.FlightNumber != "100" {
			t.Errorf("flight filter = %+v", filter)
		}

		return []model.FlightNumberRoute{{
			Origin: testCodeBOS, Dest: testCodeORD, Flights: 20, UsualDepTime: "0700",
		}}, nil
	}

	res := callTool(t, stub, toolCheckItinerary, map[string]any{
		jsonKeyLegs: []map[string]any{
			{
				jsonKeyCarrier: testCarrierUA, jsonKeyFlightNumber: "0100",
				jsonKeyDate: testDateOct6, jsonKeyDepTime: "700", jsonKeyArrTime: "905",
			},
			{
				jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyCarrier: testCarrierUA,
				jsonKeyDate: testDateOct6, jsonKeyDepTime: "1100", jsonKeyArrTime: "1400",
			},
		},
	})
	if res.IsError {
		t.Fatalf("IsError content=%s", toolText(t, res))
	}

	out := decodeResult[checkItineraryData](t, res)
	leg := out.Data.Legs[0]

	if leg.Outlook == nil || leg.Outlook.Filters.Origin != testCodeBOS || leg.Outlook.Filters.Dest != testCodeORD {
		t.Fatalf("resolved leg = %+v", leg)
	}
}

func TestCheckItineraryFlightNumberNotFound(t *testing.T) {
	stub := &storetest.Stub{
		FlightNumberRoutesFn: func(context.Context, store.FlightNumberRoutesFilter) ([]model.FlightNumberRoute, error) {
			return nil, store.ErrNotFound
		},
	}

	res := callTool(t, stub, toolCheckItinerary, map[string]any{
		jsonKeyLegs: []map[string]any{
			{
				jsonKeyCarrier: testCarrierUA, jsonKeyFlightNumber: "9999",
				jsonKeyDate: testDateOct6, jsonKeyDepTime: "700", jsonKeyArrTime: "905",
			},
			{
				jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeLAX, jsonKeyCarrier: testCarrierUA,
				jsonKeyDate: testDateOct6, jsonKeyDepTime: "1100", jsonKeyArrTime: "1400",
			},
		},
	})
	if !res.IsError {
		t.Fatal("expected error")
	}

	out := decodeResult[checkItineraryData](t, res)
	if out.Error == nil || out.Error.Code != CodeFlightNumberNotFound {
		t.Fatalf("error = %#v", out.Error)
	}
}

func TestCheckItineraryUnknownAirport(t *testing.T) {
	stub := &storetest.Stub{
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			if filter.Origin == testCodeORX {
				return nil, store.ErrNotFound
			}

			out := outlookWithConnectionMinutes(60, 90)
			out.Analysis = model.Analysis{
				SampleSize:   40,
				SampleReason: model.SampleReasonSufficient,
				Confidence:   model.ConfidenceHigh,
			}

			return out, nil
		},
		ListAirportCountriesByIATAFn: func(context.Context, []string) (map[string]string, error) {
			return map[string]string{testCodeBOS: "US", testCodeORD: "US", testCodeLAX: "US"}, nil
		},
		SearchAirportsFn: func(_ context.Context, query string, _ int) ([]model.Airport, error) {
			if query == testCodeORX {
				return []model.Airport{{IATACode: testCodeORD, Name: testNameOHare, Match: model.AirportMatchCloseCode}}, nil
			}

			return []model.Airport{{IATACode: query, Match: model.AirportMatchExactIATA}}, nil
		},
	}

	res, out, err := checkItinerary(context.Background(), stub, testLogger(), checkItineraryInput{Legs: []checkItineraryLegInput{
		{Origin: testCodeBOS, Dest: testCodeORD, Carrier: testCarrierUA, Date: testDateOct6, DepTime: "0700", ArrTime: "905"},
		{Origin: testCodeORX, Dest: testCodeLAX, Carrier: testCarrierUA, Date: testDateOct6, DepTime: "1100", ArrTime: "1400"},
	}})
	if err != nil {
		t.Fatalf("checkItinerary: %v", err)
	}

	if res == nil || !res.IsError || out.Error == nil || out.Error.Code != CodeAirportNotFound || out.Data != nil {
		t.Fatalf("error = %#v data = %#v", out.Error, out.Data)
	}

	if len(out.Error.Matches) != 1 || out.Error.Matches[0].IATACode != testCodeORD {
		t.Fatalf("matches = %#v", out.Error.Matches)
	}
}

func TestCheckItineraryKnownAirportMissingOutlook(t *testing.T) {
	stub := &storetest.Stub{
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			if filter.Dest == testCodeLAX {
				return nil, store.ErrNotFound
			}

			out := outlookWithConnectionMinutes(60, 90)
			out.Filters = model.Filters{Origin: filter.Origin, Dest: filter.Dest}
			out.Analysis = model.Analysis{
				SampleSize:   40,
				SampleReason: model.SampleReasonSufficient,
				Confidence:   model.ConfidenceHigh,
			}

			return out, nil
		},
		ListAirportCountriesByIATAFn: func(context.Context, []string) (map[string]string, error) {
			return map[string]string{testCodeBOS: "US", testCodeORD: "US", testCodeLAX: "US"}, nil
		},
		SearchAirportsFn: exactAirportSearch,
	}

	res, out, err := checkItinerary(context.Background(), stub, testLogger(), checkItineraryInput{Legs: []checkItineraryLegInput{
		{Origin: testCodeBOS, Dest: testCodeORD, Carrier: testCarrierUA, Date: testDateOct6, DepTime: "0700", ArrTime: "905"},
		{Origin: testCodeORD, Dest: testCodeLAX, Carrier: testCarrierUA, Date: testDateOct6, DepTime: "1100", ArrTime: "1400"},
	}})
	if err != nil {
		t.Fatalf("checkItinerary: %v", err)
	}

	if res == nil || res.IsError || out.Error != nil || out.Data == nil || len(out.Data.Legs) != 2 {
		t.Fatalf("error = %#v legs = %#v", out.Error, out.Data)
	}

	if out.Data.Legs[0].Outlook == nil || out.Data.Legs[0].Error != nil {
		t.Fatalf("first leg = %+v", out.Data.Legs[0])
	}

	if out.Data.Legs[1].Outlook != nil || out.Data.Legs[1].Error == nil || *out.Data.Legs[1].Error != store.ItineraryLegOutlookNotFound {
		t.Fatalf("second leg = %+v", out.Data.Legs[1])
	}
}

func TestCheckItineraryBadInput(t *testing.T) {
	valid := checkItineraryLegInput{
		Origin: testCodeBOS, Dest: testCodeORD, Carrier: testCarrierUA,
		Date: testDateOct6, DepTime: "700", ArrTime: "905",
	}
	onlyOrigin := valid
	onlyOrigin.Dest = ""
	onlyDest := valid
	onlyDest.Origin = ""

	tests := []struct {
		name string
		legs []checkItineraryLegInput
	}{
		{name: "one leg", legs: []checkItineraryLegInput{valid}},
		{name: "five legs", legs: []checkItineraryLegInput{valid, valid, valid, valid, valid}},
		{name: "only origin", legs: []checkItineraryLegInput{onlyOrigin, valid}},
		{name: "only dest", legs: []checkItineraryLegInput{onlyDest, valid}},
		{name: "bad date", legs: []checkItineraryLegInput{func() checkItineraryLegInput {
			leg := valid
			leg.Date = "2026-13-01"

			return leg
		}(), valid}},
		{name: "bad dep time", legs: []checkItineraryLegInput{func() checkItineraryLegInput {
			leg := valid
			leg.DepTime = "2460"

			return leg
		}(), valid}},
		{name: "flight number too long", legs: []checkItineraryLegInput{
			{Carrier: testCarrierUA, FlightNumber: "12345", Date: testDateOct6, DepTime: "700", ArrTime: "905"},
			valid,
		}},
		{name: "flight number not digits", legs: []checkItineraryLegInput{
			{Carrier: testCarrierUA, FlightNumber: "12A", Date: testDateOct6, DepTime: "700", ArrTime: "905"},
			valid,
		}},
		{name: "flight number empty", legs: []checkItineraryLegInput{
			{Carrier: testCarrierUA, FlightNumber: "", Date: testDateOct6, DepTime: "700", ArrTime: "905"},
			valid,
		}},
		{name: "flight number all zeros", legs: []checkItineraryLegInput{
			{Carrier: testCarrierUA, FlightNumber: "0000", Date: testDateOct6, DepTime: "700", ArrTime: "905"},
			valid,
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, out, err := checkItinerary(context.Background(), &storetest.Stub{}, testLogger(), checkItineraryInput{Legs: tt.legs})
			if err != nil {
				t.Fatalf("checkItinerary: %v", err)
			}

			if res == nil || !res.IsError || out.Error == nil || out.Error.Code != CodeInvalidInput {
				t.Fatalf("error = %#v", out.Error)
			}
		})
	}
}

func TestCheckItineraryInputSchema(t *testing.T) {
	schema := checkItineraryInputSchema()
	legs := schema.Properties[jsonKeyLegs]

	if legs == nil || legs.MinItems == nil || *legs.MinItems != checkItineraryMinLegs || legs.MaxItems == nil || *legs.MaxItems != checkItineraryMaxLegs {
		t.Fatalf("legs schema = %#v", legs)
	}

	if legs.Items == nil {
		t.Fatal("legs items schema is nil")
	}

	props := legs.Items.Properties
	assertCodeSchema(t, props["carrier"], 2)
	assertCodeSchema(t, props["origin"], 3)
	assertCodeSchema(t, props["dest"], 3)

	flightNumber := props[jsonKeyFlightNumber]
	if flightNumber == nil || flightNumber.MaxLength == nil || *flightNumber.MaxLength != store.FlightNumberRawMaxLen || flightNumber.Pattern != store.FlightNumberPattern {
		t.Fatalf("flight_number schema = %#v", flightNumber)
	}

	window := props["dep_time_window_minutes"]
	if window == nil || window.Minimum == nil || *window.Minimum != 1 || window.Maximum == nil || *window.Maximum != float64(store.MaxDepTimeWindowMinutes) {
		t.Fatalf("window schema = %#v", window)
	}

	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	leg := map[string]any{
		jsonKeyOrigin: testCodeBOS, jsonKeyDest: testCodeORD, jsonKeyCarrier: testCarrierUA,
		jsonKeyDate: testDateOct6, jsonKeyDepTime: "0700", jsonKeyArrTime: "0905",
	}
	if err := resolved.Validate(map[string]any{jsonKeyLegs: []any{leg, leg}}); err != nil {
		t.Fatalf("valid args: %v", err)
	}

	if err := resolved.Validate(map[string]any{jsonKeyLegs: []any{leg}}); err == nil {
		t.Fatal("expected validation error for one leg")
	}

	if err := resolved.Validate(map[string]any{jsonKeyLegs: []any{leg, leg, leg, leg, leg}}); err == nil {
		t.Fatal("expected validation error for five legs")
	}

	longNumber := map[string]any{
		jsonKeyOrigin: testCodeBOS, jsonKeyDest: testCodeORD, jsonKeyCarrier: testCarrierUA,
		jsonKeyFlightNumber: "12345",
		jsonKeyDate:         testDateOct6, jsonKeyDepTime: "0700", jsonKeyArrTime: "0905",
	}
	if err := resolved.Validate(map[string]any{jsonKeyLegs: []any{longNumber, leg}}); err == nil {
		t.Fatal("expected validation error for long flight number")
	}
}

func threeLegItinerary() []map[string]any {
	return []map[string]any{
		{
			jsonKeyOrigin: testCodeBOS, jsonKeyDest: testCodeORD, jsonKeyCarrier: testCarrierUA,
			jsonKeyDate: testDateOct6, jsonKeyDepTime: "700", jsonKeyArrTime: "905",
		},
		{
			jsonKeyOrigin: testCodeORD, jsonKeyDest: testCodeDFW, jsonKeyCarrier: testCarrierUA,
			jsonKeyDate: testDateOct6, jsonKeyDepTime: "940", jsonKeyArrTime: "1130",
		},
		{
			jsonKeyOrigin: testCodeDFW, jsonKeyDest: testCodeLAX, jsonKeyCarrier: testCarrierUA,
			jsonKeyDate: testDateOct6, jsonKeyDepTime: "1200", jsonKeyArrTime: "1500",
		},
	}
}

// missingRoute is "ORIGIN-DEST" when that leg's outlook is not found.
func multiLegOutlookStub(missingRoute string) *storetest.Stub {
	return &storetest.Stub{
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			if missingRoute != "" && filter.Origin+"-"+filter.Dest == missingRoute {
				return nil, store.ErrNotFound
			}

			out := outlookWithConnectionMinutes(60, 90)
			out.Filters = model.Filters{
				Origin:    filter.Origin,
				Dest:      filter.Dest,
				Carrier:   filter.Carrier,
				DayOfWeek: filter.DayOfWeek,
				DepTime:   filter.DepTime,
			}
			out.Analysis = model.Analysis{
				SampleSize:   40,
				SampleReason: model.SampleReasonSufficient,
				Confidence:   model.ConfidenceHigh,
			}

			return out, nil
		},
		ListAirportCountriesByIATAFn: func(context.Context, []string) (map[string]string, error) {
			return map[string]string{
				testCodeBOS: "US", testCodeORD: "US", testCodeDFW: "US", testCodeLAX: "US",
			}, nil
		},
		SearchAirportsFn: exactAirportSearch,
	}
}

func itineraryOutlookStub(t *testing.T) *storetest.Stub {
	return &storetest.Stub{
		RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			out := outlookWithConnectionMinutes(60, 90)
			out.Filters = model.Filters{
				Origin:               filter.Origin,
				Dest:                 filter.Dest,
				Carrier:              filter.Carrier,
				DayOfWeek:            filter.DayOfWeek,
				DepTime:              filter.DepTime,
				DepTimeWindowMinutes: filter.DepTimeWindowMinutes,
			}
			out.Analysis = model.Analysis{
				Start:        "2025-10-06",
				End:          "2026-10-06",
				SampleSize:   40,
				SampleReason: model.SampleReasonSufficient,
				Confidence:   model.ConfidenceHigh,
			}

			rate := 0.82
			out.OnTimeProbability = &rate

			return out, nil
		},
		ListAirportCountriesByIATAFn: func(context.Context, []string) (map[string]string, error) {
			return map[string]string{"BOS": "US", "ORD": "US", "LAX": "US"}, nil
		},
		SearchAirportsFn: exactAirportSearch,
	}
}

func outlookWithConnectionMinutes(recommended, _ int) *model.RouteOutlook {
	bucket := model.NewConnectionMinutes(recommended)

	return &model.RouteOutlook{
		OutlookResult: model.OutlookResult{
			Connection: model.ConnectionGuidance{
				DomesticToDomestic: bucket,
			},
		},
	}
}

func exactAirportSearch(_ context.Context, query string, _ int) ([]model.Airport, error) {
	return []model.Airport{{
		IATACode: query,
		Match:    model.AirportMatchExactIATA,
	}}, nil
}

func callTool(t *testing.T, s store.Store, name string, args map[string]any) *mcpsdk.CallToolResult {
	t.Helper()

	session := connectServer(t, s)

	res, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	return res
}
