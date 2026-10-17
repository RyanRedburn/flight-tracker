package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func normalizeCarrier(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func resolveRouteFromFlightNumber(
	ctx context.Context,
	s store.Store,
	carrier, flightNumber, depTime string,
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
			Message: "Flight number is required.",
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

	if depTime != "" {
		filtered := filterFlightRoutesByDepTime(routes, depTime, windowMinutes)
		if len(filtered) == 1 {
			return filtered[0].Origin, filtered[0].Dest, nil
		}

		if len(filtered) > 1 {
			routes = filtered
		}
	}

	return "", "", &Failure{
		Code:         CodeFlightNumberAmbiguous,
		Message:      fmt.Sprintf("%s %s flies more than one route recently. Pick origin and destination from the list, or add dep_time.", carrier, flightNumber),
		FlightRoutes: routes,
	}
}

func filterFlightRoutesByDepTime(routes []model.FlightNumberRoute, depTime string, windowMinutes int) []model.FlightNumberRoute {
	target, ok := store.HHMMToMinutes(depTime)
	if !ok {
		return routes
	}

	if windowMinutes <= 0 {
		windowMinutes = store.DefaultDepTimeWindowMinutes
	}

	out := make([]model.FlightNumberRoute, 0, len(routes))

	for _, route := range routes {
		usual, ok := store.HHMMToMinutes(route.UsualDepTime)
		if !ok {
			continue
		}

		delta := absCircularMinuteDelta(target, usual)
		if delta <= windowMinutes {
			out = append(out, route)
		}
	}

	return out
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
