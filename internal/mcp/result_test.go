package mcp

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestResultJSON(t *testing.T) {
	body, err := json.Marshal(Result[struct{}]{
		Summary:    "UA is steadier on this route, but the sample is thin.",
		Confidence: model.ConfidenceLow,
		Error: &Failure{
			Code:    CodeAirportNotFound,
			Message: "No airport has IATA code \"ORX\".",
			Matches: []model.Airport{{IATACode: testCodeORD, Name: testNameOHare}},
		},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"summary", "confidence", "error"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing %s in %s", key, body)
		}
	}

	errObj, ok := got["error"].(map[string]any)
	if !ok {
		t.Fatalf("error = %#v", got["error"])
	}

	for _, key := range []string{"code", "message", "matches"} {
		if _, ok := errObj[key]; !ok {
			t.Errorf("missing error.%s in %s", key, body)
		}
	}
}

func TestCallToolTextMatchesStructuredContent(t *testing.T) {
	successStub := &storetest.Stub{
		SearchAirportsFn: func(context.Context, string, int) ([]model.Airport, error) {
			return []model.Airport{{
				IATACode: testCodeORD,
				Name:     testNameOHare,
				Match:    model.AirportMatchExactIATA,
			}}, nil
		},
	}

	success := callTool(t, successStub, toolFindAirport, map[string]any{findAirportArgQuery: testCodeORD})
	if success.IsError {
		t.Fatalf("success IsError: %s", toolText(t, success))
	}

	assertTextJSONEqualsStructured(t, success)

	errorStub := &storetest.Stub{
		SearchAirportsFn: func(context.Context, string, int) ([]model.Airport, error) {
			return nil, nil
		},
	}

	failure := callTool(t, errorStub, toolFindAirport, map[string]any{findAirportArgQuery: "Nowhereville"})
	if !failure.IsError {
		t.Fatal("expected error")
	}

	assertTextJSONEqualsStructured(t, failure)
}

func assertTextJSONEqualsStructured(t *testing.T, res *mcpsdk.CallToolResult) {
	t.Helper()

	var fromText any
	if err := json.Unmarshal([]byte(toolText(t, res)), &fromText); err != nil {
		t.Fatalf("text json: %v\n%s", err, toolText(t, res))
	}

	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured: %v", err)
	}

	var fromStructured any
	if err := json.Unmarshal(raw, &fromStructured); err != nil {
		t.Fatalf("unmarshal structured: %v; raw=%s", err, raw)
	}

	if !reflect.DeepEqual(fromText, fromStructured) {
		t.Fatalf("text %s != structured %s", toolText(t, res), raw)
	}
}
