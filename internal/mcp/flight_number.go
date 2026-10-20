package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const msgCarrierCode = "Carrier must be a 2-letter marketing airline code."

func normalizeCarrier(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func resolveRouteFromFlightNumber(
	ctx context.Context,
	s store.Store,
	logger *slog.Logger,
	tool, carrier, flightNumber string,
	depTimes []string,
	origin, dest string,
	windowMinutes int,
) (string, string, bool, *Failure) {
	carrier = normalizeCarrier(carrier)
	if len(carrier) != 2 {
		return "", "", false, &Failure{
			Code:    CodeInvalidInput,
			Message: msgCarrierCode,
		}
	}

	origin, dest, supplied, fail := normalizeSuppliedRoute(origin, dest)
	if fail != nil {
		return "", "", false, fail
	}

	if supplied {
		return origin, dest, false, nil
	}

	flightNumber, err := store.NormalizeFlightNumber(flightNumber)
	if err != nil {
		return "", "", false, &Failure{
			Code:    CodeInvalidInput,
			Message: "Flight number is required and must be 1 to 4 digits.",
		}
	}

	routes, err := s.FlightNumberRoutes(ctx, store.FlightNumberRoutesFilter{
		Carrier:      carrier,
		FlightNumber: flightNumber,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return "", "", false, &Failure{
				Code:    CodeFlightNumberNotFound,
				Message: fmt.Sprintf("No recent history for %s %s. Check the carrier and flight number.", carrier, flightNumber),
			}
		}

		logger.Error(tool, "stage", "flight_number", "error", err)

		return "", "", false, &Failure{
			Code:    CodeUnavailable,
			Message: "Flight-number lookup is unavailable. Try again.",
		}
	}

	if len(routes) == 1 {
		return routes[0].Origin, routes[0].Dest, true, nil
	}

	filtered, near, matchedTimes := routesNearDepTimes(routes, depTimes, windowMinutes)

	switch len(filtered) {
	case 1:
		return filtered[0].Origin, filtered[0].Dest, true, nil
	case 0:
		return "", "", false, &Failure{
			Code:         CodeFlightNumberAmbiguous,
			Message:      fmt.Sprintf("No %s %s route leaves near %s; recent routes are listed. Pick origin and destination.", carrier, flightNumber, near),
			FlightRoutes: routes,
		}
	default:
		return "", "", false, &Failure{
			Code:         CodeFlightNumberAmbiguous,
			Message:      fmt.Sprintf("%s %s has more than one route near %s. Pick origin and destination.", carrier, flightNumber, joinAnd(matchedTimes)),
			FlightRoutes: filtered,
		}
	}
}

// routesNearDepTimes keeps routes whose usual departure is within the window of any given time.
// near is empty when no time could be parsed. matchedTimes are the given times that hit a route.
func routesNearDepTimes(routes []model.FlightNumberRoute, depTimes []string, windowMinutes int) ([]model.FlightNumberRoute, string, []string) {
	if windowMinutes <= 0 {
		windowMinutes = store.DefaultDepTimeWindowMinutes
	}

	targets := make([]int, 0, len(depTimes))
	labels := make([]string, 0, len(depTimes))

	for _, dep := range depTimes {
		target, ok := store.HHMMToMinutes(dep)
		if !ok {
			continue
		}

		targets = append(targets, target)
		labels = append(labels, store.FormatHHMM(dep))
	}

	if len(targets) == 0 {
		return nil, "", nil
	}

	out := make([]model.FlightNumberRoute, 0, len(routes))
	hit := make([]bool, len(targets))

	for _, route := range routes {
		usual, ok := store.HHMMToMinutes(route.UsualDepTime)
		if !ok {
			continue
		}

		matched := false

		for i, target := range targets {
			if absCircularMinuteDelta(target, usual) <= windowMinutes {
				matched = true
				hit[i] = true
			}
		}

		if matched {
			out = append(out, route)
		}
	}

	matchedTimes := make([]string, 0, len(labels))
	for i, label := range labels {
		if hit[i] {
			matchedTimes = append(matchedTimes, label)
		}
	}

	return out, joinAnd(labels), matchedTimes
}

func absCircularMinuteDelta(a, b int) int {
	delta := a - b
	if delta < 0 {
		delta = -delta
	}

	if alt := 24*60 - delta; alt < delta {
		delta = alt
	}

	return delta
}
