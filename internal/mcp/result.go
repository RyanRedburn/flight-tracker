package mcp

import (
	"github.com/RyanRedburn/flight-tracker/internal/model"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	CodeInvalidInput    = "invalid_input"
	CodeAirportNotFound = "airport_not_found"
	CodeUnavailable     = "unavailable"
)

// Result is the structured output every tool returns.
// Summary is one recommendation line and its caveat.
// Confidence, SampleSize, and SampleReason use the decision vocabulary when the
// tool is backed by flight history. A lookup omits the sample.
type Result[T any] struct {
	Summary      string   `json:"summary,omitempty"`
	Confidence   string   `json:"confidence,omitempty"`
	SampleSize   *int     `json:"sample_size,omitempty"`
	SampleReason string   `json:"sample_reason,omitempty"`
	Data         *T       `json:"data,omitempty"`
	Error        *Failure `json:"error,omitempty"`
}

// Failure is a tool error an assistant can act on.
// Matches are close airports when a code or name is unknown.
// AvailableStart and AvailableEnd are the dates that have data when the requested window does not.
type Failure struct {
	Code           string          `json:"code"`
	Message        string          `json:"message"`
	Matches        []model.Airport `json:"matches,omitempty"`
	AvailableStart string          `json:"available_start,omitempty"`
	AvailableEnd   string          `json:"available_end,omitempty"`
}

func successResult[T any](summary, confidence string, sampleSize *int, sampleReason string, data *T) (*mcpsdk.CallToolResult, Result[T], error) {
	return &mcpsdk.CallToolResult{}, Result[T]{
		Summary:      summary,
		Confidence:   confidence,
		SampleSize:   sampleSize,
		SampleReason: sampleReason,
		Data:         data,
	}, nil
}

func failureResult[T any](failure *Failure) (*mcpsdk.CallToolResult, Result[T], error) {
	summary := ""
	if failure != nil {
		summary = failure.Message
	}

	var content []mcpsdk.Content
	if summary != "" {
		content = []mcpsdk.Content{&mcpsdk.TextContent{Text: summary}}
	}

	return &mcpsdk.CallToolResult{IsError: true, Content: content}, Result[T]{
		Summary: summary,
		Error:   failure,
	}, nil
}
