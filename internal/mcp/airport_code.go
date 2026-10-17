package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func normalizeIATACode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

func resolveAirportCode(ctx context.Context, s store.Store, field, code string) (string, *Failure) {
	code = normalizeIATACode(code)
	if len(code) != 3 {
		return "", &Failure{
			Code:    CodeInvalidInput,
			Message: field + " must be a 3-letter IATA airport code.",
		}
	}

	airports, err := s.SearchAirports(ctx, code, store.AirportSearchMaxLimit)
	if err != nil {
		return "", &Failure{
			Code:    CodeUnavailable,
			Message: "Airport lookup is unavailable. Try again.",
		}
	}

	for _, airport := range airports {
		if airport.Match == model.AirportMatchExactIATA || strings.EqualFold(airport.IATACode, code) {
			return airport.IATACode, nil
		}
	}

	if len(airports) > 0 && allCloseCodes(airports) {
		return "", &Failure{
			Code:    CodeAirportNotFound,
			Message: closeMatchMessage(code, airports),
			Matches: airports,
		}
	}

	if len(airports) == 0 {
		return "", &Failure{
			Code:    CodeAirportNotFound,
			Message: fmt.Sprintf("No airport has IATA code %q. Use find_airport to search by city or name.", code),
		}
	}

	return "", &Failure{
		Code: CodeAirportNotFound,
		Message: fmt.Sprintf("No airport has IATA code %q. Similar matches include %s (%s).",
			code, airports[0].IATACode, airports[0].Name),
		Matches: airports,
	}
}
