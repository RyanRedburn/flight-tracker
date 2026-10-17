package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"time"

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
)

type routeReliabilityInput struct {
	Origin               string   `json:"origin,omitempty" jsonschema:"Origin IATA code"`
	Dest                 string   `json:"dest,omitempty" jsonschema:"Destination IATA code"`
	Carrier              string   `json:"carrier" jsonschema:"Marketing carrier code"`
	FlightNumber         string   `json:"flight_number,omitempty" jsonschema:"Marketing flight number when origin and destination are unknown"`
	Date                 string   `json:"date,omitempty" jsonschema:"Travel date (YYYY-MM-DD); use this or day_of_week"`
	DayOfWeek            int      `json:"day_of_week,omitempty" jsonschema:"Weekday 1=Mon … 7=Sun; use this or date"`
	DepTimes             []string `json:"dep_times" jsonschema:"Scheduled departure times (hhmm) to compare"`
	StartDate            string   `json:"start_date,omitempty" jsonschema:"Stats range start (YYYY-MM-DD); defaults to the outlook lookback"`
	EndDate              string   `json:"end_date,omitempty" jsonschema:"Stats range end (YYYY-MM-DD); defaults to the latest history"`
	DepTimeWindowMinutes *int     `json:"dep_time_window_minutes,omitempty" jsonschema:"Minutes around each dep_time for outlook history (1–120)"`
}

type routeReliabilityDeparture struct {
	DepTime string             `json:"dep_time"`
	Outlook model.RouteOutlook `json:"outlook"`
}

type routeReliabilityData struct {
	Stats      model.RouteStats            `json:"stats"`
	Departures []routeReliabilityDeparture `json:"departures"`
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
	schema.Properties["flight_number"].MaxLength = jsonschema.Ptr(store.FlightNumberMaxLen)
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
		Description: "Route performance stats plus booking outlook for one or more departure times on the same day. Use when comparing which flight time is more reliable.",
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
			Message: "Carrier must be a 2-letter marketing airline code.",
		})
	}

	if len(in.DepTimes) < routeReliabilityMinDepTimes || len(in.DepTimes) > routeReliabilityMaxDepTimes {
		return failureResult[routeReliabilityData](&Failure{
			Code:    CodeInvalidInput,
			Message: "dep_times must list from " + strconv.Itoa(routeReliabilityMinDepTimes) + " to " + strconv.Itoa(routeReliabilityMaxDepTimes) + " departure times.",
		})
	}

	window := store.ResolveDepTimeWindowMinutes(in.DepTimeWindowMinutes)

	origin, dest, fail := resolveRouteFromFlightNumber(ctx, s, carrier, in.FlightNumber, firstDepTime(in.DepTimes), in.Origin, in.Dest, window)
	if fail != nil {
		return failureResult[routeReliabilityData](fail)
	}

	origin, fail = resolveAirportCode(ctx, s, "origin", origin)
	if fail != nil {
		return failureResult[routeReliabilityData](fail)
	}

	dest, fail = resolveAirportCode(ctx, s, "dest", dest)
	if fail != nil {
		return failureResult[routeReliabilityData](fail)
	}

	day, fail := resolveOutlookDay(in.Date, in.DayOfWeek)
	if fail != nil {
		return failureResult[routeReliabilityData](fail)
	}

	depTimes := make([]string, 0, len(in.DepTimes))
	for _, dep := range in.DepTimes {
		dep = strings.TrimSpace(dep)
		if _, ok := store.HHMMToMinutes(dep); !ok {
			return failureResult[routeReliabilityData](&Failure{
				Code:    CodeInvalidInput,
				Message: "Each dep_times entry must be a valid local time (hhmm).",
			})
		}

		depTimes = append(depTimes, dep)
	}

	departures := make([]routeReliabilityDeparture, 0, len(depTimes))

	var (
		analysisStart string
		analysisEnd   string
	)

	for _, dep := range depTimes {
		outlook, err := s.RouteOutlook(ctx, store.RouteOutlookFilter{
			Origin:               origin,
			Dest:                 dest,
			Carrier:              carrier,
			DayOfWeek:            day,
			DepTime:              store.FormatHHMM(dep),
			DepTimeWindowMinutes: window,
		})
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return failureResult[routeReliabilityData](&Failure{
					Code:    CodeRouteNotFound,
					Message: fmt.Sprintf(routeNotFoundMessage, carrier, origin, dest),
				})
			}

			logger.Error(toolRouteReliability, "stage", "outlook", "error", err)

			return failureResult[routeReliabilityData](&Failure{
				Code:    CodeUnavailable,
				Message: "Route outlook is unavailable. Try again.",
			})
		}

		if analysisStart == "" {
			analysisStart = outlook.Analysis.Start
			analysisEnd = outlook.Analysis.End
		}

		departures = append(departures, routeReliabilityDeparture{
			DepTime: store.FormatHHMM(dep),
			Outlook: *outlook,
		})
	}

	startDate, endDate, fail := resolveStatsWindow(in.StartDate, in.EndDate, analysisStart, analysisEnd)
	if fail != nil {
		return failureResult[routeReliabilityData](fail)
	}

	stats, err := s.RouteStats(ctx, store.RouteStatsFilter{
		Origin:     origin,
		Dest:       dest,
		StartDate:  startDate,
		EndDate:    endDate,
		Carrier:    carrier,
		DaysOfWeek: []int{day},
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return failureResult[routeReliabilityData](&Failure{
				Code:    CodeRouteNotFound,
				Message: fmt.Sprintf(routeNotFoundMessage, carrier, origin, dest),
			})
		}

		logger.Error(toolRouteReliability, "stage", "stats", "error", err)

		return failureResult[routeReliabilityData](&Failure{
			Code:    CodeUnavailable,
			Message: "Route stats are unavailable. Try again.",
		})
	}

	if stats.Flights == 0 {
		dates, err := s.RouteFlightDateRange(ctx, origin, dest, carrier)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return failureResult[routeReliabilityData](&Failure{
					Code:    CodeRouteNotFound,
					Message: fmt.Sprintf(routeNotFoundMessage, carrier, origin, dest),
				})
			}

			logger.Error(toolRouteReliability, "stage", "date_range", "error", err)

			return failureResult[routeReliabilityData](&Failure{
				Code:    CodeUnavailable,
				Message: "Route stats are unavailable. Try again.",
			})
		}

		return failureResult[routeReliabilityData](&Failure{
			Code: CodeNoDataInWindow,
			Message: fmt.Sprintf("No flights for %s %s–%s between %s and %s. Retry with dates from %s through %s.",
				carrier, origin, dest, startDate, endDate, dates.Start, dates.End),
			AvailableStart: dates.Start,
			AvailableEnd:   dates.End,
		})
	}

	summary, confidence, sampleSize, sampleReason := routeReliabilitySummary(carrier, origin, dest, departures, stats)

	return successResult(summary, confidence, sampleSize, sampleReason, &routeReliabilityData{
		Stats:      *stats,
		Departures: departures,
	})
}

func resolveOutlookDay(date string, dayOfWeek int) (int, *Failure) {
	date = strings.TrimSpace(date)
	hasDate := date != ""
	hasDay := dayOfWeek != 0

	if hasDate == hasDay {
		return 0, &Failure{
			Code:    CodeInvalidInput,
			Message: "Set exactly one of date or day_of_week.",
		}
	}

	if hasDay {
		if dayOfWeek < 1 || dayOfWeek > 7 {
			return 0, &Failure{
				Code:    CodeInvalidInput,
				Message: "day_of_week must be from 1 to 7.",
			}
		}

		return dayOfWeek, nil
	}

	day, ok := store.ISOWeekdayFromDate(date)
	if !ok {
		return 0, &Failure{
			Code:    CodeInvalidInput,
			Message: "date must be a valid date (YYYY-MM-DD).",
		}
	}

	return day, nil
}

func resolveStatsWindow(startIn, endIn, defaultStart, defaultEnd string) (string, string, *Failure) {
	startIn = strings.TrimSpace(startIn)
	endIn = strings.TrimSpace(endIn)

	if startIn == "" && endIn == "" {
		return defaultStart, defaultEnd, nil
	}

	if startIn == "" || endIn == "" {
		return "", "", &Failure{
			Code:    CodeInvalidInput,
			Message: "start_date and end_date must both be set, or both omitted.",
		}
	}

	start, err := time.Parse("2006-01-02", startIn)
	if err != nil {
		return "", "", &Failure{
			Code:    CodeInvalidInput,
			Message: "start_date must be a valid date (YYYY-MM-DD).",
		}
	}

	end, err := time.Parse("2006-01-02", endIn)
	if err != nil {
		return "", "", &Failure{
			Code:    CodeInvalidInput,
			Message: "end_date must be a valid date (YYYY-MM-DD).",
		}
	}

	if end.Before(start) {
		return "", "", &Failure{
			Code:    CodeInvalidInput,
			Message: "end_date must be on or after start_date.",
		}
	}

	span := int(end.Sub(start).Hours()/24) + 1
	if span > store.MaxStatsSpanDays {
		return "", "", &Failure{
			Code:    CodeInvalidInput,
			Message: "The stats date range must be at most " + strconv.Itoa(store.MaxStatsSpanDays) + " days.",
		}
	}

	return startIn, endIn, nil
}

func firstDepTime(depTimes []string) string {
	if len(depTimes) == 0 {
		return ""
	}

	return strings.TrimSpace(depTimes[0])
}

func routeReliabilitySummary(carrier, origin, dest string, departures []routeReliabilityDeparture, stats *model.RouteStats) (string, string, *int, string) {
	bestDep, bestRate, canRank := bestDepartureByOnTime(departures)
	confidence := minOutlookConfidence(departures)
	sampleSize := stats.Analysis.SampleSize
	sampleReason := stats.Analysis.SampleReason

	routeLabel := fmt.Sprintf("%s %s–%s", carrier, origin, dest)

	if len(departures) == 1 {
		return fmt.Sprintf("%s at %s is the only time compared; see on-time odds in the outlook.", routeLabel, departures[0].DepTime), confidence, &sampleSize, sampleReason
	}

	if !canRank || bestDep == "" {
		return fmt.Sprintf("On-time odds for %s differ by time, but samples are too thin to rank them.", routeLabel), confidence, &sampleSize, sampleReason
	}

	if len(departures) == 2 {
		other := departures[0].DepTime
		if other == bestDep {
			other = departures[1].DepTime
		}

		otherRate, ok := outlookOnTimeRate(departures[0].Outlook)
		if departures[0].DepTime == bestDep {
			otherRate, ok = outlookOnTimeRate(departures[1].Outlook)
		}

		if ok {
			return fmt.Sprintf("Prefer %s over %s on %s (%.0f%% vs %.0f%% on-time in outlook history).",
				bestDep, other, routeLabel, bestRate*100, otherRate*100), confidence, &sampleSize, sampleReason
		}
	}

	return fmt.Sprintf("%s is the strongest departure time on %s among the times you listed.", bestDep, routeLabel), confidence, &sampleSize, sampleReason
}

func bestDepartureByOnTime(departures []routeReliabilityDeparture) (string, float64, bool) {
	var (
		bestDep  string
		bestRate float64 = -1
		seen     bool
	)

	for _, dep := range departures {
		rate, ok := outlookOnTimeRate(dep.Outlook)
		if !ok {
			continue
		}

		seen = true

		if rate > bestRate {
			bestRate = rate
			bestDep = dep.DepTime
		}
	}

	return bestDep, bestRate, seen
}

func outlookOnTimeRate(outlook model.RouteOutlook) (float64, bool) {
	if outlook.OnTimeProbability == nil {
		return 0, false
	}

	return *outlook.OnTimeProbability, true
}

func minOutlookConfidence(departures []routeReliabilityDeparture) string {
	confidence := model.ConfidenceHigh

	for _, dep := range departures {
		leg := dep.Outlook.DecisionConfidence()
		if confidenceRankOutlook(leg) < confidenceRankOutlook(confidence) {
			confidence = leg
		}
	}

	return confidence
}

func confidenceRankOutlook(confidence string) int {
	switch confidence {
	case model.ConfidenceHigh:
		return 2
	case model.ConfidenceLow:
		return 1
	default:
		return 0
	}
}
