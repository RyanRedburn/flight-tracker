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
	Month        int     `json:"month"`
	OnTimeRate   float64 `json:"on_time_rate"`
	Reliability  string  `json:"reliability"`
	Confidence   string  `json:"confidence"`
	SampleSize   int     `json:"sample_size"`
	SampleReason string  `json:"sample_reason"`
}

type bestTravelWindowData struct {
	Origin  string        `json:"origin"`
	Dest    string        `json:"dest"`
	Carrier string        `json:"carrier,omitempty"`
	Start   string        `json:"start"`
	End     string        `json:"end"`
	Months  []travelMonth `json:"months"`
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

	data := newBestTravelWindowData(origin, dest, carrier, windows)
	summary, confidence := travelWindowSummary(data)

	return successResult(summary, confidence, data)
}

func newBestTravelWindowData(origin, dest, carrier string, windows *model.RouteTravelWindows) *bestTravelWindowData {
	months := make([]travelMonth, 0, len(windows.ByMonth))
	for _, bucket := range windows.ByMonth {
		months = append(months, travelMonthFrom(bucket))
	}

	return &bestTravelWindowData{
		Origin:  origin,
		Dest:    dest,
		Carrier: carrier,
		Start:   windows.Analysis.Start,
		End:     windows.Analysis.End,
		Months:  months,
	}
}

func travelMonthFrom(bucket model.TravelWindowMonthBucket) travelMonth {
	confidence := bucket.Confidence
	if confidence == "" {
		confidence = model.ConfidenceUnknown
	}

	return travelMonth{
		Month:        bucket.Month,
		OnTimeRate:   bucket.OnTimeRate,
		Reliability:  bucket.Reliability,
		Confidence:   confidence,
		SampleSize:   bucket.Flights,
		SampleReason: sampleReasonForConfidence(confidence),
	}
}

func travelWindowSummary(data *bestTravelWindowData) (string, string) {
	route := routeLabel(data.Carrier, data.Origin, data.Dest)
	if len(data.Months) == 0 {
		return emptyWindowSummary("month-of-year history", route, data.Start, data.End), model.ConfidenceUnknown
	}

	items := make([]scoredItem, len(data.Months))
	for i, month := range data.Months {
		items[i] = scoredItem{
			label:      monthLabel(month.Month),
			percent:    onTimePercent(month.OnTimeRate),
			sampleSize: month.SampleSize,
			confidence: month.Confidence,
		}
	}

	return comparisonSummary(nounMonth, route, items)
}
