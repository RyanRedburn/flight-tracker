package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	testCodeORD     = "ORD"
	testCodeMDW     = "MDW"
	testCityChicago = "Chicago"
	testNameOHare   = "Chicago O'Hare International Airport"
	testCountryUS   = "US"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestFindAirport(t *testing.T) {
	ord := model.Airport{
		IATACode:         testCodeORD,
		Name:             testNameOHare,
		Municipality:     testCityChicago,
		ISOCountry:       testCountryUS,
		Type:             "large_airport",
		ScheduledService: "yes",
		Match:            model.AirportMatchExactIATA,
	}
	mdw := model.Airport{
		IATACode:         testCodeMDW,
		Name:             "Chicago Midway International Airport",
		Municipality:     testCityChicago,
		ISOCountry:       testCountryUS,
		Type:             "large_airport",
		ScheduledService: "yes",
		Match:            model.AirportMatchCity,
	}
	orf := model.Airport{
		IATACode: "ORF",
		Name:     "Norfolk International Airport",
		Match:    model.AirportMatchCloseCode,
	}

	tests := []struct {
		name       string
		query      string
		limit      int
		airports   []model.Airport
		searchErr  error
		wantCall   bool
		wantLimit  int
		wantQuery  string
		wantError  bool
		wantCode   string
		wantConf   string
		wantSubstr []string
		absent     string
	}{
		{
			name:       "exact code",
			query:      " ord ",
			airports:   []model.Airport{ord},
			wantCall:   true,
			wantLimit:  store.AirportSearchDefaultLimit,
			wantQuery:  "ord",
			wantConf:   model.ConfidenceHigh,
			wantSubstr: []string{testCodeORD, testCityChicago, summaryIdentifiesOnly},
		},
		{
			name:  "city matches",
			query: testCityChicago,
			limit: 5,
			airports: []model.Airport{
				{
					IATACode:     testCodeORD,
					Name:         testNameOHare,
					Municipality: testCityChicago,
					ISOCountry:   testCountryUS,
					Match:        model.AirportMatchCity,
				},
				mdw,
			},
			wantCall:   true,
			wantLimit:  5,
			wantQuery:  testCityChicago,
			wantConf:   model.ConfidenceLow,
			wantSubstr: []string{testCodeORD, testCodeMDW, summaryPickCode},
		},
		{
			name:       "single name match",
			query:      "Midway",
			airports:   []model.Airport{func() model.Airport { a := mdw; a.Match = model.AirportMatchName; return a }()},
			wantCall:   true,
			wantLimit:  store.AirportSearchDefaultLimit,
			wantQuery:  "Midway",
			wantConf:   model.ConfidenceLow,
			wantSubstr: []string{testCodeMDW, summaryConfirmCode},
		},
		{
			name:       "no match",
			query:      "Zzzville",
			airports:   []model.Airport{},
			wantCall:   true,
			wantLimit:  store.AirportSearchDefaultLimit,
			wantQuery:  "Zzzville",
			wantError:  true,
			wantCode:   CodeAirportNotFound,
			wantSubstr: []string{"No airport matches"},
		},
		{
			name:  "close codes",
			query: "ORX",
			airports: []model.Airport{
				{IATACode: testCodeORD, Name: testNameOHare, Match: model.AirportMatchCloseCode},
				orf,
			},
			wantCall:  true,
			wantLimit: store.AirportSearchDefaultLimit,
			wantQuery: "ORX",
			wantError: true,
			wantCode:  CodeAirportNotFound,
			wantSubstr: []string{
				"Close matches",
				testCodeORD,
				"ORF",
			},
		},
		{
			name:       "empty query",
			query:      "   ",
			wantError:  true,
			wantCode:   CodeInvalidInput,
			wantSubstr: []string{"Query is required"},
		},
		{
			name:       "limit too high",
			query:      testCodeORD,
			limit:      store.AirportSearchMaxLimit + 1,
			wantError:  true,
			wantCode:   CodeInvalidInput,
			wantSubstr: []string{"Limit must be from 1 to 25"},
		},
		{
			name:      "store failure",
			query:     testCodeORD,
			searchErr: errors.New("pq: password authentication failed"),
			wantCall:  true,
			wantLimit: store.AirportSearchDefaultLimit,
			wantQuery: testCodeORD,
			wantError: true,
			wantCode:  CodeUnavailable,
			wantSubstr: []string{
				"Airport search is unavailable",
			},
			absent: "password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				gotQuery string
				gotLimit int
			)

			called := false
			stub := &storetest.Stub{
				SearchAirportsFn: func(_ context.Context, query string, limit int) ([]model.Airport, error) {
					called = true
					gotQuery = query
					gotLimit = limit

					if tt.searchErr != nil {
						return nil, tt.searchErr
					}

					return tt.airports, nil
				},
			}

			res := callFindAirport(t, stub, map[string]any{"query": tt.query, "limit": tt.limit})
			if res.IsError != tt.wantError {
				t.Fatalf("IsError = %v, want %v; content=%s", res.IsError, tt.wantError, toolText(t, res))
			}

			if called != tt.wantCall {
				t.Fatalf("store called = %v, want %v", called, tt.wantCall)
			}

			if tt.wantCall && (gotQuery != tt.wantQuery || gotLimit != tt.wantLimit) {
				t.Fatalf("search query=%q limit=%d, want %q %d", gotQuery, gotLimit, tt.wantQuery, tt.wantLimit)
			}

			out := decodeResult[findAirportData](t, res)
			text := toolText(t, res)

			for _, substr := range tt.wantSubstr {
				if !strings.Contains(out.Summary, substr) && !strings.Contains(text, substr) {
					t.Errorf("summary %q missing %q", out.Summary, substr)
				}
			}

			if tt.absent != "" && strings.Contains(text, tt.absent) {
				t.Errorf("result leaked %q in %s", tt.absent, text)
			}

			if tt.wantError {
				if out.Error == nil || out.Error.Code != tt.wantCode {
					t.Fatalf("error = %#v, want code %s", out.Error, tt.wantCode)
				}

				if tt.name == "close codes" && len(out.Error.Matches) != 2 {
					t.Fatalf("matches = %#v", out.Error.Matches)
				}

				return
			}

			if out.Confidence != tt.wantConf {
				t.Errorf("confidence = %q, want %q", out.Confidence, tt.wantConf)
			}

			if out.Data == nil || out.Data.Query != tt.wantQuery || len(out.Data.Airports) != len(tt.airports) {
				t.Fatalf("data = %#v", out.Data)
			}

			if out.SampleSize != nil || out.SampleReason != "" {
				t.Errorf("lookup set sample size=%v reason=%q", out.SampleSize, out.SampleReason)
			}
		})
	}
}

func TestFindAirportOmittedLimit(t *testing.T) {
	var gotLimit int

	stub := &storetest.Stub{
		SearchAirportsFn: func(_ context.Context, _ string, limit int) ([]model.Airport, error) {
			gotLimit = limit

			return []model.Airport{{
				IATACode: testCodeORD,
				Name:     testNameOHare,
				Match:    model.AirportMatchExactIATA,
			}}, nil
		},
	}

	res := callFindAirport(t, stub, map[string]any{"query": testCodeORD})
	if res.IsError {
		t.Fatalf("IsError, content=%s", toolText(t, res))
	}

	if gotLimit != store.AirportSearchDefaultLimit {
		t.Fatalf("limit = %d, want %d", gotLimit, store.AirportSearchDefaultLimit)
	}
}

func TestToolsAreReadOnlyFindAirport(t *testing.T) {
	session := connectServer(t, &storetest.Stub{})
	ctx := context.Background()

	var names []string

	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			t.Fatalf("tools: %v", err)
		}

		names = append(names, tool.Name)

		if tool.Annotations == nil || !tool.Annotations.ReadOnlyHint {
			t.Errorf("%s readOnlyHint = %#v", tool.Name, tool.Annotations)
		}
	}

	if len(names) != 1 || names[0] != toolFindAirport {
		t.Fatalf("tools = %#v", names)
	}
}

func callFindAirport(t *testing.T, s store.Store, args map[string]any) *mcpsdk.CallToolResult {
	t.Helper()

	session := connectServer(t, s)

	res, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      toolFindAirport,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	return res
}

func connectServer(t *testing.T, s store.Store) *mcpsdk.ClientSession {
	t.Helper()

	ctx := context.Background()
	server := newServer(s, testLogger())
	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()

	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}

	session, err := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test", Version: "v1"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}

	t.Cleanup(func() {
		_ = session.Close()
	})

	return session
}

func decodeResult[T any](t *testing.T, res *mcpsdk.CallToolResult) Result[T] {
	t.Helper()

	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured: %v", err)
	}

	var out Result[T]
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal structured: %v; raw=%s", err, raw)
	}

	return out
}

func toolText(t *testing.T, res *mcpsdk.CallToolResult) string {
	t.Helper()

	if len(res.Content) == 0 {
		return ""
	}

	text, ok := res.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatalf("content[0] = %T", res.Content[0])
	}

	return text.Text
}
