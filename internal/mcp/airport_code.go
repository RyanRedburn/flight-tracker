package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func normalizeIATACode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func normalizeSuppliedRoute(origin, dest string) (string, string, bool, *Failure) {
	origin = normalizeIATACode(origin)
	dest = normalizeIATACode(dest)

	switch {
	case origin == "" && dest == "":
		return "", "", false, nil
	case origin == "" || dest == "":
		return "", "", false, &Failure{
			Code:    CodeInvalidInput,
			Message: "Pass both origin and destination, or pass carrier and flight_number without them.",
		}
	}

	if len(origin) != 3 {
		return "", "", false, &Failure{
			Code:    CodeInvalidInput,
			Message: "origin must be a 3-letter IATA airport code.",
		}
	}

	if len(dest) != 3 {
		return "", "", false, &Failure{
			Code:    CodeInvalidInput,
			Message: "dest must be a 3-letter IATA airport code.",
		}
	}

	return origin, dest, true, nil
}

// explainMissingRoute runs only after a route lookup found nothing.
// Codes that came from a flight number are already real routes, so they skip the airport search.
func explainMissingRoute(ctx context.Context, s store.Store, logger *slog.Logger, tool, carrier, origin, dest string, fromFlightNumber bool) *Failure {
	if fromFlightNumber {
		return routeNotFoundFailure(carrier, origin, dest)
	}

	for _, code := range []string{origin, dest} {
		airports, err := s.SearchAirports(ctx, code, store.AirportSearchMaxLimit)
		if err != nil {
			logger.Error(tool, "stage", "airport", "code", code, "error", err)

			return &Failure{
				Code:    CodeUnavailable,
				Message: "Airport lookup is unavailable. Try again.",
			}
		}

		if airportKnown(airports, code) {
			continue
		}

		return airportNotFoundFailure(code, airports)
	}

	return routeNotFoundFailure(carrier, origin, dest)
}

func routeNotFoundFailure(carrier, origin, dest string) *Failure {
	return &Failure{
		Code:    CodeRouteNotFound,
		Message: fmt.Sprintf(routeNotFoundMessage, carrier, origin, dest),
	}
}

func airportKnown(airports []model.Airport, code string) bool {
	for _, airport := range airports {
		if airport.Match == model.AirportMatchExactIATA || strings.EqualFold(airport.IATACode, code) {
			return true
		}
	}

	return false
}

func airportNotFoundFailure(code string, airports []model.Airport) *Failure {
	if len(airports) == 0 {
		return &Failure{
			Code:    CodeAirportNotFound,
			Message: fmt.Sprintf("No airport has IATA code %q. Use find_airport to search by city or name.", code),
		}
	}

	return &Failure{
		Code:    CodeAirportNotFound,
		Message: closeMatchMessage(code, airports),
		Matches: airports,
	}
}

func closeMatchMessage(query string, airports []model.Airport) string {
	limit := len(airports)
	if limit > 3 {
		limit = 3
	}

	parts := make([]string, 0, limit)
	for _, airport := range airports[:limit] {
		if airport.Name == "" {
			parts = append(parts, airport.IATACode)
			continue
		}

		parts = append(parts, airport.IATACode+" ("+airport.Name+")")
	}

	return fmt.Sprintf("No airport has IATA code %q. Close matches: %s. Search again with one of those codes or a city name.", query, strings.Join(parts, ", "))
}
