package mcp

import (
	"github.com/RyanRedburn/flight-tracker/internal/model"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	CodeInvalidInput          = "invalid_input"
	CodeAirportNotFound       = "airport_not_found"
	CodeFlightNumberNotFound  = "flight_number_not_found"
	CodeFlightNumberAmbiguous = "flight_number_ambiguous"
	CodeRouteNotFound         = "route_not_found"
	CodeUnavailable           = "unavailable"
)

// Result is the structured output every tool returns.
// Summary is one recommendation line.
// Confidence uses the decision vocabulary when the tool is backed by flight history.
type Result[T any] struct {
	Summary    string   `json:"summary,omitempty"`
	Confidence string   `json:"confidence,omitempty"`
	Data       *T       `json:"data,omitempty"`
	Error      *Failure `json:"error,omitempty"`
}

// Failure is a tool error an assistant can act on.
// Matches are close airports when a code or name is unknown.
// FlightRoutes lists recent routes for a marketing flight number when the AI must pick one.
type Failure struct {
	Code         string                    `json:"code"`
	Message      string                    `json:"message"`
	Matches      []model.Airport           `json:"matches,omitempty"`
	FlightRoutes []model.FlightNumberRoute `json:"flight_routes,omitempty"`
}

func successResult[T any](summary, confidence string, data *T) (*mcpsdk.CallToolResult, Result[T], error) {
	return &mcpsdk.CallToolResult{}, Result[T]{
		Summary:    summary,
		Confidence: confidence,
		Data:       data,
	}, nil
}

func failureResult[T any](failure *Failure) (*mcpsdk.CallToolResult, Result[T], error) {
	summary := ""
	if failure != nil {
		summary = failure.Message
	}

	// Content stays nil so the SDK writes this same JSON into the text block.
	return &mcpsdk.CallToolResult{IsError: true}, Result[T]{
		Summary: summary,
		Error:   failure,
	}, nil
}
