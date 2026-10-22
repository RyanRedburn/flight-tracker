package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"

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
		Month:        bucket.Month,
		OnTimeRate:   bucket.OnTimeRate,
		Reliability:  bucket.Reliability,
		Confidence:   bucket.Confidence,
		SampleSize:   bucket.Flights,
		SampleReason: sampleReason(bucket.Flights, bucket.Confidence),
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

	ranked, thin, none := splitByConfidence(items)
	if len(ranked) == 0 {
		return appendNote("No month for "+route+" has enough data to compare.", thinSentence(thin), noneSentence(none)), comparisonConfidence(ranked, thin, none)
	}

	return appendNote(bestWorstMonths(route, ranked), thinSentence(thin), noneSentence(none)), comparisonConfidence(ranked, thin, none)
}

// bestWorstMonths compares only the high-confidence months. Thin months are named after it.
func bestWorstMonths(route string, ranked []scoredItem) string {
	bestPct, worstPct := ranked[0].percent, ranked[0].percent
	minFlights, maxFlights := ranked[0].sampleSize, ranked[0].sampleSize

	for _, item := range ranked[1:] {
		bestPct = max(bestPct, item.percent)
		worstPct = min(worstPct, item.percent)
		minFlights = min(minFlights, item.sampleSize)
		maxFlights = max(maxFlights, item.sampleSize)
	}

	if len(ranked) == 1 {
		return fmt.Sprintf("%s is the only month with enough data for %s (%d%% on time). High confidence, %s.",
			ranked[0].label, route, bestPct, flightsPhrase([]int{minFlights}))
	}

	lead := spreadMonths(route, ranked, bestPct, worstPct)
	if bestPct-worstPct < routeReliabilitySamePointGap {
		lead = sameMonths(route, ranked, bestPct, worstPct)
	}

	return lead + " " + monthCountSentence(len(ranked), minFlights, maxFlights)
}

// sameMonths is for a spread under the shared same-point gap, so no month is called best or worst.
func sameMonths(route string, ranked []scoredItem, bestPct, worstPct int) string {
	rates := joinItemPercents(ranked)
	if bestPct == worstPct {
		rates = strconv.Itoa(bestPct) + "%"
	}

	return fmt.Sprintf("%s are about the same for %s (%s on time).", joinLabels(ranked), route, rates)
}

// spreadMonths names every month tied for best and every month tied for worst.
func spreadMonths(route string, ranked []scoredItem, bestPct, worstPct int) string {
	best := make([]scoredItem, 0, len(ranked))
	worst := make([]scoredItem, 0, len(ranked))

	for _, item := range ranked {
		if item.percent == bestPct {
			best = append(best, item)
		}

		if item.percent == worstPct {
			worst = append(worst, item)
		}
	}

	bestNoun := "month"
	if len(best) > 1 {
		bestNoun = "months"
	}

	return fmt.Sprintf("%s %s the most reliable %s for %s (%d%% on time), and %s %s the least reliable (%d%%).",
		joinLabels(best), pluralVerb(len(best)), bestNoun, route, bestPct, joinLabels(worst), pluralVerb(len(worst)), worstPct)
}

// monthCountSentence covers only the months with enough data, not the thin ones named after it.
func monthCountSentence(months, minFlights, maxFlights int) string {
	flights := strconv.Itoa(minFlights)
	if minFlights != maxFlights {
		flights += " to " + strconv.Itoa(maxFlights)
	}

	return fmt.Sprintf("High confidence. The %d months with enough data have %s flights each.", months, flights)
}
