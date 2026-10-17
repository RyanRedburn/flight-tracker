package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const (
	fieldDepTime  = "dep_time"
	fieldDepTimes = "dep_times"
)

func normalizeCarrier(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func resolveRouteFromFlightNumber(
	ctx context.Context,
	s store.Store,
	carrier, flightNumber string,
	depTimes []string,
	depTimeField string,
	origin, dest string,
	windowMinutes int,
) (string, string, *Failure) {
	carrier = normalizeCarrier(carrier)
	if len(carrier) != 2 {
		return "", "", &Failure{
			Code:    CodeInvalidInput,
			Message: "Carrier must be a 2-letter marketing airline code.",
		}
	}

	origin = normalizeIATACode(origin)
	dest = normalizeIATACode(dest)

	if origin != "" && dest != "" {
		return origin, dest, nil
	}

	if origin != "" || dest != "" {
		return "", "", &Failure{
			Code:    CodeInvalidInput,
			Message: "Pass both origin and destination, or pass carrier and flight_number without them.",
		}
	}

	flightNumber, ok := store.NormalizeFlightNumber(flightNumber)
	if !ok {
		return "", "", &Failure{
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
			return "", "", &Failure{
				Code:    CodeFlightNumberNotFound,
				Message: fmt.Sprintf("No recent history for %s %s. Check the carrier and flight number.", carrier, flightNumber),
			}
		}

		return "", "", &Failure{
			Code:    CodeUnavailable,
			Message: "Flight-number lookup is unavailable. Try again.",
		}
	}

	if len(routes) == 1 {
		return routes[0].Origin, routes[0].Dest, nil
	}

	filtered, near := routesNearDepTimes(routes, depTimes, windowMinutes)
	if near != "" {
		switch len(filtered) {
		case 1:
			return filtered[0].Origin, filtered[0].Dest, nil
		case 0:
			return "", "", &Failure{
				Code:         CodeFlightNumberAmbiguous,
				Message:      fmt.Sprintf("No %s %s route leaves near %s; recent routes are listed. Pick origin and destination.", carrier, flightNumber, near),
				FlightRoutes: routes,
			}
		default:
			routes = filtered
		}
	}

	return "", "", &Failure{
		Code:         CodeFlightNumberAmbiguous,
		Message:      fmt.Sprintf("%s %s flies more than one route recently. Pick origin and destination from the list, or add %s.", carrier, flightNumber, depTimeField),
		FlightRoutes: routes,
	}
}

// routesNearDepTimes keeps routes whose usual departure is within the window of any given time.
// near is empty when no time could be parsed, so the caller keeps the full list.
func routesNearDepTimes(routes []model.FlightNumberRoute, depTimes []string, windowMinutes int) ([]model.FlightNumberRoute, string) {
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
		return nil, ""
	}

	out := make([]model.FlightNumberRoute, 0, len(routes))

	for _, route := range routes {
		usual, ok := store.HHMMToMinutes(route.UsualDepTime)
		if !ok {
			continue
		}

		for _, target := range targets {
			if absCircularMinuteDelta(target, usual) <= windowMinutes {
				out = append(out, route)
				break
			}
		}
	}

	return out, joinAnd(labels)
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
