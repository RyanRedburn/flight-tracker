package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolFindAirport = "find_airport"

	summaryIdentifiesOnly = "This identifies the airport only, not a reliability score."
	summaryConfirmCode    = "Confirm this code; it is not a reliability score."
	summaryPickCode       = "Pick the IATA code for the trip; this is not a reliability ranking."
)

type findAirportInput struct {
	Query string `json:"query" jsonschema:"City, airport name, or IATA code"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum matches to return, from 1 to 25"`
}

type findAirportData struct {
	Query    string          `json:"query"`
	Airports []model.Airport `json:"airports"`
}

func addFindAirport(server *mcpsdk.Server, s store.Store, logger *slog.Logger) {
	closedWorld := false
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        toolFindAirport,
		Description: "Look up airports by IATA code, city, or name. Returns ranked matches. Use this when the user names a city or airport instead of a code.",
		Annotations: &mcpsdk.ToolAnnotations{
			Title:         "Find airport",
			ReadOnlyHint:  true,
			OpenWorldHint: &closedWorld,
		},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in findAirportInput) (*mcpsdk.CallToolResult, Result[findAirportData], error) {
		return findAirport(ctx, s, logger, in)
	})
}

func findAirport(ctx context.Context, s store.Store, logger *slog.Logger, in findAirportInput) (*mcpsdk.CallToolResult, Result[findAirportData], error) {
	query := strings.Join(strings.Fields(in.Query), " ")
	if query == "" {
		return failureResult[findAirportData](&Failure{
			Code:    CodeInvalidInput,
			Message: "Query is required. Pass a city, airport name, or IATA code.",
		})
	}

	if len(query) > store.AirportSearchMaxQueryLen {
		return failureResult[findAirportData](&Failure{
			Code:    CodeInvalidInput,
			Message: "Query must be at most " + strconv.Itoa(store.AirportSearchMaxQueryLen) + " characters.",
		})
	}

	limit := in.Limit
	if limit == 0 {
		limit = store.AirportSearchDefaultLimit
	}

	if limit < 1 || limit > store.AirportSearchMaxLimit {
		return failureResult[findAirportData](&Failure{
			Code:    CodeInvalidInput,
			Message: "Limit must be from 1 to " + strconv.Itoa(store.AirportSearchMaxLimit) + ".",
		})
	}

	airports, err := s.SearchAirports(ctx, query, limit)
	if err != nil {
		logger.Error(toolFindAirport, "query", query, "error", err)

		return failureResult[findAirportData](&Failure{
			Code:    CodeUnavailable,
			Message: "Airport search is unavailable. Try again.",
		})
	}

	if len(airports) == 0 {
		return failureResult[findAirportData](&Failure{
			Code:    CodeAirportNotFound,
			Message: fmt.Sprintf("No airport matches %q. Search by city, airport name, or a 3-letter IATA code.", query),
		})
	}

	if allCloseCodes(airports) {
		return failureResult[findAirportData](&Failure{
			Code:    CodeAirportNotFound,
			Message: closeMatchMessage(query, airports),
			Matches: airports,
		})
	}

	summary, confidence := airportSummary(query, airports)

	return successResult(summary, confidence, nil, "", &findAirportData{
		Query:    query,
		Airports: airports,
	})
}

func allCloseCodes(airports []model.Airport) bool {
	for _, airport := range airports {
		if airport.Match != model.AirportMatchCloseCode {
			return false
		}
	}

	return true
}

func airportSummary(query string, airports []model.Airport) (string, string) {
	exact := 0

	var exactAirport model.Airport

	for _, airport := range airports {
		if airport.Match == model.AirportMatchExactIATA || strings.EqualFold(airport.IATACode, query) {
			exact++
			exactAirport = airport
		}
	}

	if exact == 1 {
		if len(airports) == 1 {
			return fmt.Sprintf("%s is %s. Confidence is %s. %s", exactAirport.IATACode, airportPlace(exactAirport), model.ConfidenceHigh, summaryIdentifiesOnly), model.ConfidenceHigh
		}

		return fmt.Sprintf("%s is %s. Other airports also match %q; use %s unless you meant one of them. Confidence is %s. %s",
			exactAirport.IATACode, airportPlace(exactAirport), query, exactAirport.IATACode, model.ConfidenceHigh, summaryIdentifiesOnly), model.ConfidenceHigh
	}

	return ambiguousAirportSummary(query, airports), model.ConfidenceLow
}

func ambiguousAirportSummary(query string, airports []model.Airport) string {
	first := airports[0]
	if len(airports) == 1 {
		return fmt.Sprintf("%s (%s) is the only match for %q. Confidence is %s. %s",
			first.IATACode, airportPlace(first), query, model.ConfidenceLow, summaryConfirmCode)
	}

	second := airports[1]

	return fmt.Sprintf("%d airports match %q, including %s (%s) and %s (%s). Confidence is %s. %s",
		len(airports), query, first.IATACode, first.Name, second.IATACode, second.Name, model.ConfidenceLow, summaryPickCode)
}

func airportPlace(airport model.Airport) string {
	name := airport.Name
	if name == "" {
		name = airport.IATACode
	}

	city := strings.TrimSpace(airport.Municipality)
	country := strings.TrimSpace(airport.ISOCountry)

	switch {
	case city != "" && country != "":
		return name + " in " + city + ", " + country
	case city != "":
		return name + " in " + city
	case country != "":
		return name + " in " + country
	default:
		return name
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
