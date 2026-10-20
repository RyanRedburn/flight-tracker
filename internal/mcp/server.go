package mcp

import (
	"log/slog"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/store"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const serverInstructions = "Read-only. Resolve a city or airport name to an IATA code with find_airport before other questions. check_itinerary and route_reliability accept carrier plus flight_number when airports are unknown; when flight_routes lists more than one route, pick origin and destination. Each tool result has a one-line summary and a confidence value. A tool error includes close airport matches or flight route choices; use those instead of guessing."

// NewHandler is the stateless streamable HTTP handler mounted at /mcp.
// Each request is independent, so any replica can serve it.
func NewHandler(s store.Store, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	server := newServer(s, logger)

	handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server {
		return server
	}, &mcpsdk.StreamableHTTPOptions{
		Stateless:    true,
		JSONResponse: true,
		Logger:       logger,
	})

	return http.NewCrossOriginProtection().Handler(handler)
}

func newServer(s store.Store, logger *slog.Logger) *mcpsdk.Server {
	server := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "flight-tracker",
		Version: "v1",
	}, &mcpsdk.ServerOptions{
		Instructions: serverInstructions,
		Logger:       logger,
	})
	addFindAirport(server, s, logger)
	addCheckItinerary(server, s, logger)
	addRouteReliability(server, s, logger)

	return server
}
