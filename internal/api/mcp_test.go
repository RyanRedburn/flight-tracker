package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

const pathMCP = "/mcp"

func TestMCPStatelessAcrossReplicas(t *testing.T) {
	stub := &storetest.Stub{
		SearchAirportsFn: func(_ context.Context, query string, _ int) ([]model.Airport, error) {
			if query != "ORD" {
				t.Errorf("query = %q", query)
			}

			return []model.Airport{{
				IATACode: "ORD",
				Name:     "Chicago O'Hare International Airport",
				Match:    model.AirportMatchExactIATA,
			}}, nil
		},
	}

	replicaA := newRouter(stub, testLogger(), 24, nil, Security{Disabled: true})
	replicaB := newRouter(stub, testLogger(), 24, nil, Security{Disabled: true})

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"find_airport","arguments":{"query":"ORD"}}}`

	for _, replica := range []struct {
		name    string
		handler http.Handler
		session string
	}{
		{name: "replica A", handler: replicaA},
		{name: "replica B ignores foreign session", handler: replicaB, session: "session-from-replica-a"},
	} {
		t.Run(replica.name, func(t *testing.T) {
			rec := postMCP(replica.handler, body, replica.session)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}

			if got := rec.Header().Get("Mcp-Session-Id"); got != "" {
				t.Fatalf("Mcp-Session-Id = %q", got)
			}

			result := decodeMCPResult(t, rec.Body.Bytes())
			if result.IsError {
				t.Fatalf("isError body=%s", rec.Body.String())
			}

			if result.Structured == nil || result.Structured["summary"] == nil {
				t.Fatalf("structured = %#v", result.Structured)
			}

			summary, _ := result.Structured["summary"].(string)
			if !strings.Contains(summary, "ORD") {
				t.Fatalf("summary = %q", summary)
			}
		})
	}
}

func TestMCPAuthAndRateLimit(t *testing.T) {
	consumerPlain, consumerStub := stubWithAPIKey(t, model.APIKeyRoleConsumer)
	subscriberPlain, subscriberStub := stubWithAPIKey(t, model.APIKeyRoleSubscriber)
	adminPlain, adminStub := stubWithAPIKey(t, model.APIKeyRoleAdmin)

	var (
		bucket string
		rpm    int
	)

	sec := Security{Limits: testRateLimits()}
	listBody := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`

	t.Run("missing key", func(t *testing.T) {
		var failBucket string

		consumerStub.ConsumeRateLimitFn = func(_ context.Context, key string, requestsPerMinute int) (store.RateLimitResult, error) {
			failBucket = key

			return store.RateLimitResult{Allowed: true, Limit: requestsPerMinute, Remaining: 1, ResetUnix: 1}, nil
		}

		handler := newRouter(consumerStub, testLogger(), 24, nil, sec)
		req := httptest.NewRequest(http.MethodPost, pathMCP, strings.NewReader(listBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
		}

		if failBucket != "ip|192.0.2.1|"+store.RateLimitSurfaceAuthFail {
			t.Fatalf("bucket = %q", failBucket)
		}
	})

	t.Run("consumer bucket", func(t *testing.T) {
		consumerStub.ConsumeRateLimitFn = func(_ context.Context, key string, requestsPerMinute int) (store.RateLimitResult, error) {
			bucket = key
			rpm = requestsPerMinute

			return store.RateLimitResult{Allowed: true, Limit: requestsPerMinute, Remaining: requestsPerMinute - 1, ResetUnix: 1}, nil
		}

		handler := newRouter(consumerStub, testLogger(), 24, nil, sec)
		req := mcpRequest(listBody, "")
		req.Header.Set("Authorization", "Bearer "+consumerPlain)

		got := httptest.NewRecorder()
		handler.ServeHTTP(got, req)

		if got.Code != http.StatusOK {
			t.Fatalf("status = %d, body=%s", got.Code, got.Body.String())
		}

		if bucket != "key|key-consumer|"+store.RateLimitSurfaceMCP {
			t.Fatalf("bucket = %q", bucket)
		}

		if rpm != 120 {
			t.Fatalf("rpm = %d, want 120", rpm)
		}

		names := toolNames(t, got.Body.Bytes())

		want := map[string]bool{
			"find_airport":      true,
			"check_itinerary":   true,
			"route_reliability": true,
		}
		if len(names) != len(want) {
			t.Fatalf("tools = %#v", names)
		}

		for _, name := range names {
			if !want[name] {
				t.Fatalf("unexpected tool %q in %#v", name, names)
			}
		}
	})

	for _, tt := range []struct {
		name string
		stub *storetest.Stub
		key  string
	}{
		{name: "subscriber", stub: subscriberStub, key: subscriberPlain},
		{name: "admin", stub: adminStub, key: adminPlain},
	} {
		t.Run(tt.name, func(t *testing.T) {
			handler := newRouter(tt.stub, testLogger(), 24, nil, sec)
			req := mcpRequest(listBody, "")
			req.Header.Set("Authorization", "Bearer "+tt.key)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMCPRateLimitDenied(t *testing.T) {
	plain, stub := stubWithAPIKey(t, model.APIKeyRoleConsumer)
	stub.ConsumeRateLimitFn = func(context.Context, string, int) (store.RateLimitResult, error) {
		return store.RateLimitResult{Allowed: false, Limit: 120, Remaining: 0, ResetUnix: 99, RetryAfter: 5 * time.Second}, nil
	}

	handler := newRouter(stub, testLogger(), 24, nil, Security{Limits: testRateLimits()})
	req := mcpRequest(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "")
	req.Header.Set("Authorization", "Bearer "+plain)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func TestMCPGetIsMethodNotAllowed(t *testing.T) {
	handler := newRouter(&storetest.Stub{}, testLogger(), 24, nil, Security{Disabled: true})
	req := httptest.NewRequest(http.MethodGet, pathMCP, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
}

func postMCP(handler http.Handler, body, sessionID string) *httptest.ResponseRecorder {
	req := mcpRequest(body, sessionID)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec
}

func mcpRequest(body, sessionID string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, pathMCP, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}

	return req
}

type mcpCallResult struct {
	IsError    bool           `json:"isError"`
	Structured map[string]any `json:"structuredContent"`
}

func decodeMCPResult(t *testing.T, body []byte) mcpCallResult {
	t.Helper()

	var rpc struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result mcpCallResult `json:"result"`
	}
	if err := json.Unmarshal(body, &rpc); err != nil {
		t.Fatalf("decode: %v body=%s", err, body)
	}

	if rpc.Error != nil {
		t.Fatalf("rpc error: %s", rpc.Error.Message)
	}

	return rpc.Result
}

func toolNames(t *testing.T, body []byte) []string {
	t.Helper()

	var rpc struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &rpc); err != nil {
		t.Fatalf("decode tools: %v body=%s", err, body)
	}

	names := make([]string, 0, len(rpc.Result.Tools))
	for _, tool := range rpc.Result.Tools {
		names = append(names, tool.Name)
	}

	return names
}
