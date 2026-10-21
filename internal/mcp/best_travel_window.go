package mcp

import (
	"context"
	"errors"
	"log/slog"
	"reflect"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolBestTravelWindow = "best_travel_window"

	msgTravelWindowsUnavailable = "Travel windows are unavailable. Try again."
)

type bestTravelWindowInput struct {
	Origin  string `json:"origin" jsonschema:"Origin IATA code"`
	Dest    string `json:"dest" jsonschema:"Destination IATA code"`
	Carrier string `json:"carrier,omitempty" jsonschema:"Optional marketing carrier code"`
}

type travelMonth struct {
	model.TravelWindowMonthBucket

	SampleSize   int    `json:"sample_size"`
	SampleReason string `json:"sample_reason"`
}

type bestTravelWindowData struct {
	Filters     model.Filters `json:"filters"`
	Start       string        `json:"start"`
	End         string        `json:"end"`
	ByMonth     []travelMonth `json:"by_month"`
	BestMonths  []travelMonth `json:"best_months"`
	WorstMonths []travelMonth `json:"worst_months"`
}

func bestTravelWindowInputSchema() *jsonschema.Schema {
	schema, err := jsonschema.ForType(reflect.TypeFor[bestTravelWindowInput](), &jsonschema.ForOptions{})
	if err != nil {
		panic("best_travel_window input schema: " + err.Error())
	}

	limitExact(schema.Properties["origin"], 3, patternIATACode)
	limitExact(schema.Properties["dest"], 3, patternIATACode)
	limitExact(schema.Properties["carrier"], 2, patternCarrierCode)

	return schema
}

func addBestTravelWindow(server *mcpsdk.Server, s store.Store, logger *slog.Logger) {
	closedWorld := false
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        toolBestTravelWindow,
		Description: "Typical-year on-time rates by month for a route. Use when choosing which time of year to fly. Departure-time comparison belongs in route_reliability.",
		InputSchema: bestTravelWindowInputSchema(),
		Annotations: &mcpsdk.ToolAnnotations{
			Title:         "Best travel window",
			ReadOnlyHint:  true,
			OpenWorldHint: &closedWorld,
		},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in bestTravelWindowInput) (*mcpsdk.CallToolResult, Result[bestTravelWindowData], error) {
		return bestTravelWindow(ctx, s, logger, in)
	})
}

func bestTravelWindow(ctx context.Context, s store.Store, logger *slog.Logger, in bestTravelWindowInput) (*mcpsdk.CallToolResult, Result[bestTravelWindowData], error) {
	origin, dest, fail := requireRoute(in.Origin, in.Dest)
	if fail != nil {
		return failureResult[bestTravelWindowData](fail)
	}

	carrier, fail := optionalCarrier(in.Carrier)
	if fail != nil {
		return failureResult[bestTravelWindowData](fail)
	}

	windows, err := s.RouteTravelWindows(ctx, store.RouteTravelWindowsFilter{
		Origin:  origin,
		Dest:    dest,
		Carrier: carrier,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return failureResult[bestTravelWindowData](explainMissingRoute(ctx, s, logger, toolBestTravelWindow, carrier, origin, dest, false))
		}

		logger.Error(toolBestTravelWindow, "stage", "windows", "error", err)

		return failureResult[bestTravelWindowData](&Failure{
			Code:    CodeUnavailable,
			Message: msgTravelWindowsUnavailable,
		})
	}

	data := newBestTravelWindowData(windows)
	summary, confidence := travelWindowSummary(carrier, origin, dest, data)

	return successResult(summary, confidence, data)
}

func newBestTravelWindowData(windows *model.RouteTravelWindows) *bestTravelWindowData {
	return &bestTravelWindowData{
		Filters:     windows.Filters,
		Start:       windows.Analysis.Start,
		End:         windows.Analysis.End,
		ByMonth:     travelMonths(windows.ByMonth),
		BestMonths:  travelMonths(windows.BestMonths),
		WorstMonths: travelMonths(windows.WorstMonths),
	}
}

func travelMonths(buckets []model.TravelWindowMonthBucket) []travelMonth {
	months := make([]travelMonth, 0, len(buckets))
	for _, bucket := range buckets {
		months = append(months, travelMonthFrom(bucket))
	}

	return months
}

func travelMonthFrom(bucket model.TravelWindowMonthBucket) travelMonth {
	return travelMonth{
		TravelWindowMonthBucket: bucket,
		SampleSize:              bucket.Flights,
		SampleReason:            sampleReason(bucket.Flights, bucket.Confidence),
	}
}

func travelWindowSummary(carrier, origin, dest string, data *bestTravelWindowData) (string, string) {
	route := routeLabel(carrier, origin, dest)
	if len(data.ByMonth) == 0 {
		return emptyWindowSummary("month-of-year history", route, data.Start, data.End), model.ConfidenceUnknown
	}

	items := make([]scoredItem, len(data.ByMonth))
	for i, month := range data.ByMonth {
		items[i] = scoredItem{
			label:      monthLabel(month.Month),
			percent:    onTimePercent(month.OnTimeRate),
			sampleSize: month.SampleSize,
			confidence: month.Confidence,
		}
	}

	return comparisonSummary(nounMonth, route, items, true)
}
