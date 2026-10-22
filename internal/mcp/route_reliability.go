package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolRouteReliability = "route_reliability"

	routeReliabilityMinDepTimes = 1
	routeReliabilityMaxDepTimes = 5

	routeNotFoundMessage = "No history for %s %s–%s. Check the airports and carrier."

	msgOutlookSchedule = "Exactly one of date or day_of_week is required."
)

type routeReliabilityInput struct {
	Origin               string   `json:"origin,omitempty" jsonschema:"Origin IATA code"`
	Dest                 string   `json:"dest,omitempty" jsonschema:"Destination IATA code"`
	Carrier              string   `json:"carrier" jsonschema:"Marketing carrier code"`
	FlightNumber         string   `json:"flight_number,omitempty" jsonschema:"Marketing flight number when origin and destination are unknown. 1 to 4 digits; leading zeros are ignored"`
	Date                 string   `json:"date,omitempty" jsonschema:"Travel date (YYYY-MM-DD); use this or day_of_week"`
	DayOfWeek            int      `json:"day_of_week,omitempty" jsonschema:"Weekday 1=Mon … 7=Sun; use this or date"`
	DepTimes             []string `json:"dep_times" jsonschema:"Scheduled departure times (hhmm) to compare"`
	DepTimeWindowMinutes *int     `json:"dep_time_window_minutes,omitempty" jsonschema:"Minutes around each dep_time for history (1–120)"`
}

// routeDepartureOutlook is the time-specific outlook. Route fields stay on the stats object.
// Connection guidance is left out; this tool does not judge a connection.
type routeDepartureOutlook struct {
	Filters                         model.Filters             `json:"filters"`
	Analysis                        model.Analysis            `json:"analysis"`
	OnTimeProbability               *float64                  `json:"on_time_probability"`
	DelayProbability                *float64                  `json:"delay_probability"`
	CancellationProbability         *float64                  `json:"cancellation_probability"`
	DiversionProbability            *float64                  `json:"diversion_probability"`
	LikelyArrivalDelayMinutes       *float64                  `json:"likely_arrival_delay_minutes"`
	MedianArrivalDelayMinutes       *float64                  `json:"median_arrival_delay_minutes"`
	LikelyArrivalDelayWhenDelayed   *float64                  `json:"likely_arrival_delay_when_delayed"`
	MedianArrivalDelayWhenDelayed   *float64                  `json:"median_arrival_delay_when_delayed"`
	LikelyDepartureDelayMinutes     *float64                  `json:"likely_departure_delay_minutes"`
	MedianDepartureDelayMinutes     *float64                  `json:"median_departure_delay_minutes"`
	LikelyDepartureDelayWhenDelayed *float64                  `json:"likely_departure_delay_when_delayed"`
	MedianDepartureDelayWhenDelayed *float64                  `json:"median_departure_delay_when_delayed"`
	Weather                         model.RouteOutlookWeather `json:"weather"`
}

type routeReliabilityData struct {
	Stats      model.RouteStats        `json:"stats"`
	Departures []routeDepartureOutlook `json:"departures"`
}

func routeReliabilityInputSchema() *jsonschema.Schema {
	schema, err := jsonschema.ForType(reflect.TypeFor[routeReliabilityInput](), &jsonschema.ForOptions{})
	if err != nil {
		panic("route_reliability input schema: " + err.Error())
	}

	schema.Properties["carrier"].MinLength = jsonschema.Ptr(2)
	schema.Properties["carrier"].MaxLength = jsonschema.Ptr(2)
	schema.Properties["origin"].MinLength = jsonschema.Ptr(3)
	schema.Properties["origin"].MaxLength = jsonschema.Ptr(3)
	schema.Properties["dest"].MinLength = jsonschema.Ptr(3)
	schema.Properties["dest"].MaxLength = jsonschema.Ptr(3)
	schema.Properties["flight_number"].MaxLength = jsonschema.Ptr(store.FlightNumberRawMaxLen)
	schema.Properties["flight_number"].Pattern = store.FlightNumberPattern
	schema.Properties["day_of_week"].Minimum = jsonschema.Ptr(1.0)
	schema.Properties["day_of_week"].Maximum = jsonschema.Ptr(7.0)

	depTimes := schema.Properties["dep_times"]
	depTimes.MinItems = jsonschema.Ptr(routeReliabilityMinDepTimes)
	depTimes.MaxItems = jsonschema.Ptr(routeReliabilityMaxDepTimes)

	schema.Properties["dep_time_window_minutes"].Minimum = jsonschema.Ptr(1.0)
	schema.Properties["dep_time_window_minutes"].Maximum = jsonschema.Ptr(float64(store.MaxDepTimeWindowMinutes))

	return schema
}

func addRouteReliability(server *mcpsdk.Server, s store.Store, logger *slog.Logger) {
	closedWorld := false
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        toolRouteReliability,
		Description: "Route performance stats and booking outlook for one or more departure times on the same day. Stats use the same dates as the outlook. Use when comparing which flight time is more reliable.",
		InputSchema: routeReliabilityInputSchema(),
		Annotations: &mcpsdk.ToolAnnotations{
			Title:         "Route reliability",
			ReadOnlyHint:  true,
			OpenWorldHint: &closedWorld,
		},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in routeReliabilityInput) (*mcpsdk.CallToolResult, Result[routeReliabilityData], error) {
		return routeReliability(ctx, s, logger, in)
	})
}

func routeReliability(ctx context.Context, s store.Store, logger *slog.Logger, in routeReliabilityInput) (*mcpsdk.CallToolResult, Result[routeReliabilityData], error) {
	carrier := normalizeCarrier(in.Carrier)
	if len(carrier) != 2 {
		return failureResult[routeReliabilityData](&Failure{
			Code:    CodeInvalidInput,
			Message: msgCarrierCode,
		})
	}

	if len(in.DepTimes) < routeReliabilityMinDepTimes || len(in.DepTimes) > routeReliabilityMaxDepTimes {
		return failureResult[routeReliabilityData](&Failure{
			Code:    CodeInvalidInput,
			Message: "dep_times must list from " + strconv.Itoa(routeReliabilityMinDepTimes) + " to " + strconv.Itoa(routeReliabilityMaxDepTimes) + " departure times.",
		})
	}

	if in.DepTimeWindowMinutes != nil {
		if err := store.DepTimeWindowError(*in.DepTimeWindowMinutes); err != nil {
			return failureResult[routeReliabilityData](invalidInput(err))
		}
	}

	day, err := store.ParseOutlookDay(in.Date, in.DayOfWeek)
	if err != nil {
		return failureResult[routeReliabilityData](invalidInput(err))
	}

	window := store.ResolveDepTimeWindowMinutes(in.DepTimeWindowMinutes)
	depTimes := make([]string, 0, len(in.DepTimes))
	seenDep := make(map[string]struct{}, len(in.DepTimes))

	for _, dep := range in.DepTimes {
		dep = strings.TrimSpace(dep)
		if err := (store.RouteOutlookFilter{DayOfWeek: day, DepTime: dep, DepTimeWindowMinutes: window}).Validate(); err != nil {
			return failureResult[routeReliabilityData](depTimeFailure(err))
		}

		dep = store.FormatHHMM(dep)
		if _, ok := seenDep[dep]; ok {
			return failureResult[routeReliabilityData](&Failure{
				Code:    CodeInvalidInput,
				Message: "Each dep_times entry must be a different time.",
			})
		}

		seenDep[dep] = struct{}{}
		depTimes = append(depTimes, dep)
	}

	origin, dest, fromFlightNumber, fail := resolveRouteFromFlightNumber(ctx, s, logger, toolRouteReliability, carrier, in.FlightNumber, depTimes, in.Origin, in.Dest, window)
	if fail != nil {
		return failureResult[routeReliabilityData](fail)
	}

	departures := make([]routeDepartureOutlook, 0, len(depTimes))

	for _, dep := range depTimes {
		filter := store.RouteOutlookFilter{
			Origin:               origin,
			Dest:                 dest,
			Carrier:              carrier,
			DayOfWeek:            day,
			DepTime:              dep,
			DepTimeWindowMinutes: window,
			DepTimeWindowSet:     in.DepTimeWindowMinutes != nil,
		}

		outlook, err := s.RouteOutlook(ctx, filter)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return failureResult[routeReliabilityData](explainMissingRoute(ctx, s, logger, toolRouteReliability, carrier, origin, dest, fromFlightNumber))
			}

			logger.Error(toolRouteReliability, "stage", "outlook", "error", err)

			return failureResult[routeReliabilityData](&Failure{
				Code:    CodeUnavailable,
				Message: "Route outlook is unavailable. Try again.",
			})
		}

		departures = append(departures, newRouteDepartureOutlook(*outlook, filter.DepTime, window))
	}

	statsFilter := store.RouteStatsFilter{
		Origin:     origin,
		Dest:       dest,
		StartDate:  departures[0].Analysis.Start,
		EndDate:    departures[0].Analysis.End,
		Carrier:    carrier,
		DaysOfWeek: []int{day},
	}

	stats, err := s.RouteStats(ctx, statsFilter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			// The outlook already found this route, so this is not an unknown airport.
			return failureResult[routeReliabilityData](routeNotFoundFailure(carrier, origin, dest))
		}

		logger.Error(toolRouteReliability, "stage", "stats", "error", err)

		return failureResult[routeReliabilityData](&Failure{
			Code:    CodeUnavailable,
			Message: "Route stats are unavailable. Try again.",
		})
	}

	summary, confidence := routeReliabilitySummary(carrier, origin, dest, departures)

	return successResult(summary, confidence, &routeReliabilityData{
		Stats:      *stats,
		Departures: departures,
	})
}

func newRouteDepartureOutlook(out model.RouteOutlook, depTime string, window int) routeDepartureOutlook {
	return routeDepartureOutlook{
		Filters: model.Filters{
			DepTime:              depTime,
			DepTimeWindowMinutes: window,
		},
		Analysis:                        out.Analysis,
		OnTimeProbability:               out.OnTimeProbability,
		DelayProbability:                out.DelayProbability,
		CancellationProbability:         out.CancellationProbability,
		DiversionProbability:            out.DiversionProbability,
		LikelyArrivalDelayMinutes:       out.LikelyArrivalDelayMinutes,
		MedianArrivalDelayMinutes:       out.MedianArrivalDelayMinutes,
		LikelyArrivalDelayWhenDelayed:   out.LikelyArrivalDelayWhenDelayed,
		MedianArrivalDelayWhenDelayed:   out.MedianArrivalDelayWhenDelayed,
		LikelyDepartureDelayMinutes:     out.LikelyDepartureDelayMinutes,
		MedianDepartureDelayMinutes:     out.MedianDepartureDelayMinutes,
		LikelyDepartureDelayWhenDelayed: out.LikelyDepartureDelayWhenDelayed,
		MedianDepartureDelayWhenDelayed: out.MedianDepartureDelayWhenDelayed,
		Weather:                         out.Weather,
	}
}

func invalidInput(err error) *Failure {
	return &Failure{Code: CodeInvalidInput, Message: validationMessage(err)}
}

func validationMessage(err error) string {
	switch {
	case errors.Is(err, store.ErrOutlookSchedule):
		return msgOutlookSchedule
	case errors.Is(err, store.ErrInvalidDate):
		return "date must be a valid date (YYYY-MM-DD)."
	case errors.Is(err, store.ErrInvalidWeekday):
		return "day_of_week must be from 1 to 7."
	case errors.Is(err, store.ErrInvalidHHMM):
		return "dep_time must be a valid local time (hhmm)."
	default:
		return finishSentence(err.Error())
	}
}

func finishSentence(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" || strings.HasSuffix(msg, ".") {
		return msg
	}

	return msg + "."
}

func depTimeFailure(err error) *Failure {
	if errors.Is(err, store.ErrInvalidHHMM) {
		return &Failure{
			Code:    CodeInvalidInput,
			Message: "Each dep_times entry must be a valid local time (hhmm).",
		}
	}

	return invalidInput(err)
}

func routeReliabilitySummary(carrier, origin, dest string, departures []routeDepartureOutlook) (string, string) {
	route := carrier + " " + origin + "–" + dest
	confidence := departureConfidence(departures)

	if len(departures) == 1 {
		return oneDepartureSummary(route, departures[0]), confidence
	}

	return manyDepartureSummary(route, departures), confidence
}

func departureConfidence(departures []routeDepartureOutlook) string {
	ranked := make([]string, 0, len(departures))
	all := make([]string, len(departures))

	for i, dep := range departures {
		confidence := outlookConfidence(dep.Analysis)
		all[i] = confidence

		if !comparableConfidence(confidence) {
			continue
		}

		if _, ok := outlookOnTimeRate(dep.OnTimeProbability); !ok {
			continue
		}

		ranked = append(ranked, confidence)
	}

	if len(ranked) == 0 {
		return model.MinConfidence(all...)
	}

	return model.MinConfidence(ranked...)
}

func outlookConfidence(analysis model.Analysis) string {
	outlook := model.RouteOutlook{OutlookResult: model.OutlookResult{Analysis: analysis}}
	return outlook.DecisionConfidence()
}

func oneDepartureSummary(route string, dep routeDepartureOutlook) string {
	label := dep.Filters.DepTime + " on " + route
	confidence := outlookConfidence(dep.Analysis)

	if comparableConfidence(confidence) {
		rate, ok := outlookOnTimeRate(dep.OnTimeProbability)
		if !ok {
			return label + " has no reliable on-time rate."
		}

		return fmt.Sprintf("%s is on time about %d%% of the time, from %d flights.", label, onTimePercent(rate), dep.Analysis.SampleSize)
	}

	if confidence == model.ConfidenceLow {
		return fmt.Sprintf("%s has thin data (%d flights), so there is no reliable on-time rate.", label, dep.Analysis.SampleSize)
	}

	return label + " has no data for this day and time."
}

func manyDepartureSummary(route string, departures []routeDepartureOutlook) string {
	items := make([]scoredItem, len(departures))
	for i, dep := range departures {
		items[i] = departureItem(dep)
	}

	summary, _ := comparisonSummary(nounListedTime, route, items, false)

	return summary
}

func departureItem(dep routeDepartureOutlook) scoredItem {
	item := scoredItem{
		label:      dep.Filters.DepTime,
		sampleSize: dep.Analysis.SampleSize,
		confidence: outlookConfidence(dep.Analysis),
	}

	if !comparableConfidence(item.confidence) {
		return item
	}

	rate, ok := outlookOnTimeRate(dep.OnTimeProbability)
	if !ok {
		item.confidence = ""

		return item
	}

	item.rate = rate
	item.percent = onTimePercent(rate)

	return item
}

func outlookOnTimeRate(rate *float64) (float64, bool) {
	if rate == nil {
		return 0, false
	}

	return *rate, true
}

func onTimePercent(rate float64) int {
	return int(math.Round(rate * 100))
}
