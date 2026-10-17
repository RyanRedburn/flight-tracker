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
		"legs": []map[string]any{
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
	if !out.Data.Itinerary.Summary.ConnectionRisk {
		t.Fatalf("connection_risk = false")
	}

	if out.Confidence != model.ConfidenceHigh {
		t.Fatalf("confidence = %q", out.Confidence)
	}

	if out.SampleSize == nil || *out.SampleSize == 0 {
		t.Fatalf("sample_size = %v", out.SampleSize)
	}
}

func TestCheckItineraryFlightNumberAmbiguous(t *testing.T) {
	stub := &storetest.Stub{
		FlightNumberRoutesFn: func(context.Context, store.FlightNumberRoutesFilter) ([]model.FlightNumberRoute, error) {
			return []model.FlightNumberRoute{
				{Origin: testCodeORD, Dest: testCodeLAX, Flights: 40, UsualDepTime: "0700"},
				{Origin: testCodeORD, Dest: "SFO", Flights: 35, UsualDepTime: "0715"},
			}, nil
		},
	}

	res := callTool(t, stub, toolCheckItinerary, map[string]any{
		"legs": []map[string]any{
			{
				jsonKeyCarrier: testCarrierUA, "flight_number": "100",
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
