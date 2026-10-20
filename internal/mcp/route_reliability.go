package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"reflect"
	"sort"
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

	// routeReliabilitySamePointGap is the on-time gap, in percentage points. A gap under this is about the same.
	routeReliabilitySamePointGap = 3

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

type rankedDeparture struct {
	time    string
	percent int
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

		if confidence != model.ConfidenceHigh {
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

	switch outlookConfidence(dep.Analysis) {
	case model.ConfidenceHigh:
		rate, ok := outlookOnTimeRate(dep.OnTimeProbability)
		if !ok {
			return label + " has no reliable on-time rate."
		}

		return fmt.Sprintf("%s is on time about %d%% of the time, from %d flights.", label, onTimePercent(rate), dep.Analysis.SampleSize)
	case model.ConfidenceLow:
		return fmt.Sprintf("%s has thin data (%d flights), so there is no reliable on-time rate.", label, dep.Analysis.SampleSize)
	default:
		return label + " has no data for this day and time."
	}
}

func manyDepartureSummary(route string, departures []routeDepartureOutlook) string {
	var (
		ranked []rankedDeparture
		thin   []string
		none   []string
	)

	for _, dep := range departures {
		timeLabel := dep.Filters.DepTime

		switch outlookConfidence(dep.Analysis) {
		case model.ConfidenceHigh:
			rate, ok := outlookOnTimeRate(dep.OnTimeProbability)
			if !ok {
				none = append(none, timeLabel)
				continue
			}

			ranked = append(ranked, rankedDeparture{time: timeLabel, percent: onTimePercent(rate)})
		case model.ConfidenceLow:
			thin = append(thin, timeLabel)
		default:
			none = append(none, timeLabel)
		}
	}

	switch len(ranked) {
	case 0:
		return appendDataNote("No listed time on "+route+" has enough data to compare.", thin, none)
	case 1:
		return appendDataNote(fmt.Sprintf("%s is the only listed time with enough data on %s (%d%% on time).", ranked[0].time, route, ranked[0].percent), thin, none)
	default:
		return appendDataNote(rankedSummary(route, ranked), thin, none)
	}
}

func rankedSummary(route string, ranked []rankedDeparture) string {
	sorted := append([]rankedDeparture(nil), ranked...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].percent > sorted[j].percent })

	split := 1
	for split < len(sorted) && sorted[0].percent-sorted[split].percent < routeReliabilitySamePointGap {
		split++
	}

	leaders, trailers := sorted[:split], sorted[split:]

	if len(trailers) == 0 {
		return aboutTheSame(route, leaders)
	}

	// A trailer within the gap of the lowest leader is not clearly worse, so list every time in order instead.
	if leaders[len(leaders)-1].percent-trailers[0].percent < routeReliabilitySamePointGap {
		return orderedSummary(route, sorted)
	}

	if len(leaders) == 1 {
		return fmt.Sprintf("%s is more reliable than %s on %s (%d%% vs %s on time).",
			leaders[0].time, joinRankedTimes(trailers), route, leaders[0].percent, joinPercents(trailers))
	}

	return fmt.Sprintf("%s are about the same and more reliable than %s on %s (%s vs %s on time).",
		joinRankedTimes(leaders), joinRankedTimes(trailers), route, joinPercents(leaders), joinPercents(trailers))
}

// orderedSummary compares the displayed percents, so every time tied with the best leads.
func orderedSummary(route string, sorted []rankedDeparture) string {
	tied := 1
	for tied < len(sorted) && sorted[tied].percent == sorted[0].percent {
		tied++
	}

	rest := make([]string, 0, len(sorted)-tied)
	for _, item := range sorted[tied:] {
		rest = append(rest, fmt.Sprintf("%s (%d%%)", item.time, item.percent))
	}

	verb := "is"
	if tied > 1 {
		verb = "are"
	}

	return fmt.Sprintf("%s %s most reliable on %s (%d%% on time), then %s.",
		joinRankedTimes(sorted[:tied]), verb, route, sorted[0].percent, joinAnd(rest))
}

func aboutTheSame(route string, leaders []rankedDeparture) string {
	if samePercent(leaders) {
		return fmt.Sprintf("%s are about the same on %s (%d%% on time).", joinRankedTimes(leaders), route, leaders[0].percent)
	}

	return fmt.Sprintf("%s are about the same on %s (%s on time).", joinRankedTimes(leaders), route, joinPercents(leaders))
}

func appendDataNote(base string, thin, none []string) string {
	note := dataGapSentence(thin, none)
	if note == "" {
		return base
	}

	return base + " " + note
}

func dataGapSentence(thin, none []string) string {
	parts := make([]string, 0, 2)
	if len(thin) > 0 {
		parts = append(parts, joinAnd(thin)+" "+haveVerb(len(thin))+" thin data")
	}

	if len(none) > 0 {
		parts = append(parts, joinAnd(none)+" "+haveVerb(len(none))+" no data")
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " and ") + "."
}

func haveVerb(n int) string {
	if n == 1 {
		return "has"
	}

	return "have"
}

func samePercent(items []rankedDeparture) bool {
	for _, item := range items[1:] {
		if item.percent != items[0].percent {
			return false
		}
	}

	return true
}

func joinRankedTimes(items []rankedDeparture) string {
	times := make([]string, len(items))
	for i, item := range items {
		times[i] = item.time
	}

	return joinAnd(times)
}

func joinPercents(items []rankedDeparture) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = strconv.Itoa(item.percent) + "%"
	}

	return joinAnd(parts)
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
