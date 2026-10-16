package mcp

import (
	"encoding/json"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

func TestResultJSON(t *testing.T) {
	size := 12

	body, err := json.Marshal(Result[struct{}]{
		Summary:      "UA is steadier on this route, but the sample is thin.",
		Confidence:   model.ConfidenceLow,
		SampleSize:   &size,
		SampleReason: model.SampleReasonInsufficientSample,
		Error: &Failure{
			Code:           CodeAirportNotFound,
			Message:        "No flights in that window.",
			AvailableStart: "2024-01-01",
			AvailableEnd:   "2025-06-30",
			Matches:        []model.Airport{{IATACode: testCodeORD, Name: testNameOHare}},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"summary", "confidence", "sample_size", "sample_reason", "error"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing %s in %s", key, body)
		}
	}

	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %#v", got["error"])
	}

	for _, key := range []string{"code", "message", "matches", "available_start", "available_end"} {
		if _, ok := errObj[key]; !ok {
			t.Errorf("missing error.%s in %s", key, body)
		}
	}

	if got["sample_size"] != float64(size) || got["sample_reason"] != model.SampleReasonInsufficientSample {
		t.Fatalf("sample = %#v %#v", got["sample_size"], got["sample_reason"])
	}
}

func TestFailureResultTextContent(t *testing.T) {
	const msg = "Query is required. Pass a city, airport name, or IATA code."

	res, out, err := failureResult[struct{}](&Failure{
		Code:    CodeInvalidInput,
		Message: msg,
	})
	if err != nil {
		t.Fatalf("failureResult: %v", err)
	}

	if !res.IsError {
		t.Fatal("expected IsError")
	}

	if out.Error == nil || out.Error.Message != msg {
		t.Fatalf("error = %#v", out.Error)
	}

	if text := toolText(t, res); text != msg {
		t.Fatalf("text = %q, want %q", text, msg)
	}
}
