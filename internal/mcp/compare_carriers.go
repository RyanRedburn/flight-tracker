package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolCompareCarriers = "compare_carriers"

	msgCarriersUnavailable = "Carrier comparison is unavailable. Try again."
)

type compareCarriersInput struct {
	Origin    string `json:"origin" jsonschema:"Origin IATA code"`
	Dest      string `json:"dest" jsonschema:"Destination IATA code"`
	StartDate string `json:"start_date,omitempty" jsonschema:"Range start (YYYY-MM-DD). Pass with end_date, or omit both for the trailing year"`
	EndDate   string `json:"end_date,omitempty" jsonschema:"Range end (YYYY-MM-DD). Pass with start_date, or omit both for the trailing year"`
}

type comparedCarrier struct {
	Carrier      string  `json:"carrier"`
	OnTimeRate   float64 `json:"on_time_rate"`
	Confidence   string  `json:"confidence"`
	SampleSize   int     `json:"sample_size"`
	SampleReason string  `json:"sample_reason"`
}

type compareCarriersData struct {
	Stats    model.RouteStats  `json:"stats"`
	Carriers []comparedCarrier `json:"carriers"`
}

func compareCarriersInputSchema() *jsonschema.Schema {
	schema, err := jsonschema.ForType(reflect.TypeFor[compareCarriersInput](), &jsonschema.ForOptions{})
	if err != nil {
		panic("compare_carriers input schema: " + err.Error())
	}

	limitExact(schema.Properties["origin"], 3, patternIATACode)
	limitExact(schema.Properties["dest"], 3, patternIATACode)
	limitDate(schema.Properties["start_date"])
	limitDate(schema.Properties["end_date"])

	return schema
}

func addCompareCarriers(server *mcpsdk.Server, s store.Store, logger *slog.Logger) {
	closedWorld := false
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        toolCompareCarriers,
		Description: "On-time comparison of marketing carriers on one route. Use when choosing which airline is more reliable on that route. Dates are optional and default to the trailing year ending at the route's latest flight.",
		InputSchema: compareCarriersInputSchema(),
		Annotations: &mcpsdk.ToolAnnotations{
			Title:         "Compare carriers",
			ReadOnlyHint:  true,
			OpenWorldHint: &closedWorld,
		},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in compareCarriersInput) (*mcpsdk.CallToolResult, Result[compareCarriersData], error) {
		return compareCarriers(ctx, s, logger, in)
	})
}

func compareCarriers(ctx context.Context, s store.Store, logger *slog.Logger, in compareCarriersInput) (*mcpsdk.CallToolResult, Result[compareCarriersData], error) {
	origin, dest, fail := requireRoute(in.Origin, in.Dest)
	if fail != nil {
		return failureResult[compareCarriersData](fail)
	}

	start, end, fail := optionalStatsDates(in.StartDate, in.EndDate)
	if fail != nil {
		return failureResult[compareCarriersData](fail)
	}

	if start == "" {
		latest, err := s.RouteLatestFlightDate(ctx, origin, dest)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return failureResult[compareCarriersData](explainMissingRoute(ctx, s, logger, toolCompareCarriers, "", origin, dest, false))
			}

			logger.Error(toolCompareCarriers, "stage", "latest_flight", "error", err)

			return failureResult[compareCarriersData](&Failure{
				Code:    CodeUnavailable,
				Message: msgCarriersUnavailable,
			})
		}

		var ok bool

		start, end, ok = store.TrailingOutlookWindow(latest)
		if !ok {
			logger.Error(toolCompareCarriers, "stage", "window", "date", latest)

			return failureResult[compareCarriersData](&Failure{
				Code:    CodeUnavailable,
				Message: msgCarriersUnavailable,
			})
		}
	}

	stats, err := s.RouteStats(ctx, store.RouteStatsFilter{
		Origin:    origin,
		Dest:      dest,
		StartDate: start,
		EndDate:   end,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return failureResult[compareCarriersData](explainMissingRoute(ctx, s, logger, toolCompareCarriers, "", origin, dest, false))
		}

		logger.Error(toolCompareCarriers, "stage", "stats", "error", err)

		return failureResult[compareCarriersData](&Failure{
			Code:    CodeUnavailable,
			Message: msgCarriersUnavailable,
		})
	}

	data := newCompareCarriersData(stats)
	summary, confidence := compareCarriersSummary(origin, dest, data)

	return successResult(summary, confidence, data)
}

func newCompareCarriersData(stats *model.RouteStats) *compareCarriersData {
	carriers := make([]comparedCarrier, 0, len(stats.CarrierOnTime))
	for _, row := range stats.CarrierOnTime {
		carriers = append(carriers, comparedCarrierFrom(row))
	}

	trimmed := *stats
	trimmed.CarrierOnTime = nil

	return &compareCarriersData{
		Stats:    trimmed,
		Carriers: carriers,
	}
}

func comparedCarrierFrom(row model.CarrierOnTime) comparedCarrier {
	// Same history bands used to decide whether a carrier is ranked.
	confidence := model.CountConfidence(row.Flights, model.HistoryLowSampleBelow, model.HistoryHighSampleAt)

	return comparedCarrier{
		Carrier:      row.Carrier,
		OnTimeRate:   row.OnTimeRate,
		Confidence:   confidence,
		SampleSize:   row.Flights,
		SampleReason: sampleReason(row.Flights, confidence),
	}
}

func compareCarriersSummary(origin, dest string, data *compareCarriersData) (string, string) {
	route := routeLabel("", origin, dest)
	if len(data.Carriers) == 0 {
		return emptyWindowSummary("flights", route, data.Stats.Analysis.Start, data.Stats.Analysis.End), model.ConfidenceUnknown
	}

	items := make([]scoredItem, len(data.Carriers))
	for i, carrier := range data.Carriers {
		items[i] = scoredItem{
			label:      carrierLabel(carrier.Carrier),
			percent:    onTimePercent(carrier.OnTimeRate),
			rate:       carrier.OnTimeRate,
			sampleSize: carrier.SampleSize,
			confidence: carrier.Confidence,
		}
	}

	if len(items) == 1 && comparableConfidence(items[0].confidence) {
		return soleCarrierSummary(route, items[0])
	}

	return comparisonSummary(nounCarrier, route, items, true)
}

func soleCarrierSummary(route string, item scoredItem) (string, string) {
	return sentenceCase(fmt.Sprintf("%s is on time about %d%% of the time on %s (%s, %s).",
		item.label, item.percent, route, confidencePhrase(item.confidence), flightsPhrase([]int{item.sampleSize}))), item.confidence
}
