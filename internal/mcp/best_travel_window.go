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
			rate:       month.OnTimeRate,
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

// bestWorstMonths compares months with enough data. Thin months are named after it.
func bestWorstMonths(route string, ranked []scoredItem) string {
	if len(ranked) == 1 {
		return fmt.Sprintf("%s is the only month with enough data for %s (%d%% on time). %s, %s.",
			ranked[0].label, route, ranked[0].percent, sentenceCase(confidencePhrase(ranked[0].confidence)), flightsPhrase([]int{ranked[0].sampleSize}))
	}

	minFlights, maxFlights := ranked[0].sampleSize, ranked[0].sampleSize

	for _, item := range ranked[1:] {
		minFlights = min(minFlights, item.sampleSize)
		maxFlights = max(maxFlights, item.sampleSize)
	}

	return monthComparison(route, ranked) + " " + monthCountSentence(len(ranked), minFlights, maxFlights, itemConfidence(ranked))
}

// monthComparison names a best and worst month only when those rates differ at 95%.
func monthComparison(route string, ranked []scoredItem) string {
	sorted := sortByRate(ranked)
	if !anyItemDiffers(sorted) {
		return sameMonths(route, sorted)
	}

	leaders, rest := splitLeaders(sorted)
	if len(rest) > 0 && leadersSeparated(leaders, rest) && !anyItemDiffers(rest) {
		return spreadMonths(route, leaders, rest)
	}

	if len(rest) > 0 && leadersSeparated(leaders, rest) {
		return monthLeadersAhead(route, leaders, rest)
	}

	better, worse, ok := strongestPair(sorted)
	if !ok {
		return sameMonths(route, sorted)
	}

	return fmt.Sprintf("%s is the most reliable month for %s (%d%% on time), and %s is the least reliable (%d%%).",
		better.label, route, better.percent, worse.label, worse.percent)
}

func monthLeadersAhead(route string, leaders, rest []scoredItem) string {
	if len(leaders) == 1 {
		return fmt.Sprintf("%s is more reliable than %s for %s (%d%% vs %s on time).",
			leaders[0].label, joinLabels(rest), route, leaders[0].percent, joinItemPercents(rest))
	}

	rates := joinItemPercents(leaders)
	if sameItemPercent(leaders) {
		rates = strconv.Itoa(leaders[0].percent) + "%"
	}

	return fmt.Sprintf("%s are %s and more reliable than %s for %s (%s vs %s on time).",
		joinLabels(leaders), phraseTooClose, joinLabels(rest), route, rates, joinItemPercents(rest))
}

// sameMonths is the whole set inside the margin of error, so no month is called best or worst.
func sameMonths(route string, ranked []scoredItem) string {
	rates := joinItemPercents(ranked)
	if sameItemPercent(ranked) {
		rates = strconv.Itoa(ranked[0].percent) + "%"
	}

	return fmt.Sprintf("%s are %s for %s (%s on time).", joinLabels(ranked), phraseTooClose, route, rates)
}

// spreadMonths names the months too close to the best and the months too close to the worst.
func spreadMonths(route string, best, worst []scoredItem) string {
	bestNoun := "month"
	if len(best) > 1 {
		bestNoun = "months"
	}

	bestRates := strconv.Itoa(best[0].percent) + "%"
	if !sameItemPercent(best) {
		bestRates = joinItemPercents(best)
	}

	worstRates := strconv.Itoa(worst[0].percent) + "%"
	if !sameItemPercent(worst) {
		worstRates = joinItemPercents(worst)
	}

	return fmt.Sprintf("%s %s the most reliable %s for %s (%s on time), and %s %s the least reliable (%s).",
		joinLabels(best), pluralVerb(len(best)), bestNoun, route, bestRates, joinLabels(worst), pluralVerb(len(worst)), worstRates)
}

// monthCountSentence covers only the months with enough data, not the thin ones named after it.
func monthCountSentence(months, minFlights, maxFlights int, confidence string) string {
	flights := strconv.Itoa(minFlights)
	if minFlights != maxFlights {
		flights += " to " + strconv.Itoa(maxFlights)
	}

	return fmt.Sprintf("%s. The %d months with enough data have %s flights each.", sentenceCase(confidencePhrase(confidence)), months, flights)
}
