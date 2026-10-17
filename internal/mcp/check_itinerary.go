package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/google/jsonschema-go/jsonschema"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	toolCheckItinerary = "check_itinerary"

	checkItineraryMinLegs = 2
	checkItineraryMaxLegs = 4
)

type checkItineraryLegInput struct {
	Origin               string `json:"origin,omitempty" jsonschema:"Origin IATA code"`
	Dest                 string `json:"dest,omitempty" jsonschema:"Destination IATA code"`
	Carrier              string `json:"carrier" jsonschema:"Marketing carrier code"`
	FlightNumber         string `json:"flight_number,omitempty" jsonschema:"Marketing flight number when origin and destination are unknown"`
	Date                 string `json:"date" jsonschema:"Travel date (YYYY-MM-DD)"`
	DepTime              string `json:"dep_time" jsonschema:"Scheduled departure (hhmm)"`
	ArrTime              string `json:"arr_time" jsonschema:"Scheduled arrival (hhmm)"`
	DepTimeWindowMinutes *int   `json:"dep_time_window_minutes,omitempty" jsonschema:"Minutes around dep_time for outlook history (1–120)"`
}

type checkItineraryInput struct {
	Legs []checkItineraryLegInput `json:"legs" jsonschema:"Ordered flight legs, 2 to 4"`
}

type checkItineraryData struct {
	Itinerary model.ItineraryOutlookResponse `json:"itinerary"`
}

func checkItineraryInputSchema() *jsonschema.Schema {
	schema, err := jsonschema.ForType(reflect.TypeFor[checkItineraryInput](), &jsonschema.ForOptions{})
	if err != nil {
		panic("check_itinerary input schema: " + err.Error())
	}

	legs := schema.Properties["legs"]
	legs.MinItems = jsonschema.Ptr(checkItineraryMinLegs)
	legs.MaxItems = jsonschema.Ptr(checkItineraryMaxLegs)

	legProps := legs.Items.Properties
	legProps["carrier"].MinLength = jsonschema.Ptr(2)
	legProps["carrier"].MaxLength = jsonschema.Ptr(2)
	legProps["origin"].MinLength = jsonschema.Ptr(3)
	legProps["origin"].MaxLength = jsonschema.Ptr(3)
	legProps["dest"].MinLength = jsonschema.Ptr(3)
	legProps["dest"].MaxLength = jsonschema.Ptr(3)
	legProps["flight_number"].MaxLength = jsonschema.Ptr(store.FlightNumberMaxLen)
	legProps["dep_time_window_minutes"].Minimum = jsonschema.Ptr(1.0)
	legProps["dep_time_window_minutes"].Maximum = jsonschema.Ptr(float64(store.MaxDepTimeWindowMinutes))

	return schema
}

func addCheckItinerary(server *mcpsdk.Server, s store.Store, logger *slog.Logger) {
	closedWorld := false
	mcpsdk.AddTool(server, &mcpsdk.Tool{
		Name:        toolCheckItinerary,
		Description: "Historical outlook and connection risk for a 2–4 leg itinerary. Use when the traveler asks whether a connection is safe or how reliable a multi-leg trip is.",
		InputSchema: checkItineraryInputSchema(),
		Annotations: &mcpsdk.ToolAnnotations{
			Title:         "Check itinerary",
			ReadOnlyHint:  true,
			OpenWorldHint: &closedWorld,
		},
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in checkItineraryInput) (*mcpsdk.CallToolResult, Result[checkItineraryData], error) {
		return checkItinerary(ctx, s, logger, in)
	})
}

func checkItinerary(ctx context.Context, s store.Store, logger *slog.Logger, in checkItineraryInput) (*mcpsdk.CallToolResult, Result[checkItineraryData], error) {
	if len(in.Legs) < checkItineraryMinLegs || len(in.Legs) > checkItineraryMaxLegs {
		return failureResult[checkItineraryData](&Failure{
			Code:    CodeInvalidInput,
			Message: "Legs must contain from " + strconv.Itoa(checkItineraryMinLegs) + " to " + strconv.Itoa(checkItineraryMaxLegs) + " flights.",
		})
	}

	legs := make([]model.ItineraryLeg, len(in.Legs))

	for i, legIn := range in.Legs {
		window := store.ResolveDepTimeWindowMinutes(legIn.DepTimeWindowMinutes)

		origin, dest, fail := resolveRouteFromFlightNumber(ctx, s,
			legIn.Carrier, legIn.FlightNumber, []string{legIn.DepTime}, fieldDepTime,
			legIn.Origin, legIn.Dest, window)
		if fail != nil {
			return failureResult[checkItineraryData](fail)
		}

		origin, fail = resolveAirportCode(ctx, s, "origin", origin)
		if fail != nil {
			return failureResult[checkItineraryData](fail)
		}

		dest, fail = resolveAirportCode(ctx, s, "dest", dest)
		if fail != nil {
			return failureResult[checkItineraryData](fail)
		}

		legs[i] = model.ItineraryLeg{
			Origin:               origin,
			Dest:                 dest,
			Carrier:              normalizeCarrier(legIn.Carrier),
			Date:                 strings.TrimSpace(legIn.Date),
			DepTime:              strings.TrimSpace(legIn.DepTime),
			ArrTime:              strings.TrimSpace(legIn.ArrTime),
			DepTimeWindowMinutes: legIn.DepTimeWindowMinutes,
		}
	}

	req := model.ItineraryOutlookRequest{Legs: legs}
	if err := req.Validate(); err != nil {
		return failureResult[checkItineraryData](&Failure{
			Code:    CodeInvalidInput,
			Message: err.Error(),
		})
	}

	out, err := store.ItineraryOutlook(ctx, s, req.Legs)
	if err != nil {
		logger.Error(toolCheckItinerary, "error", err)

		return failureResult[checkItineraryData](&Failure{
			Code:    CodeUnavailable,
			Message: "Itinerary outlook is unavailable. Try again.",
		})
	}

	summary, confidence := itineraryToolResult(out)

	return successResult(summary, confidence, &checkItineraryData{Itinerary: out})
}

func itineraryToolResult(out model.ItineraryOutlookResponse) (string, string) {
	confidence := out.Summary.Confidence
	tight, unknown := riskyConnections(out.Connections)

	switch {
	case len(tight) > 0 && len(unknown) > 0:
		return mixedConnectionSummary(tight, unknown), confidence
	case len(tight) > 1:
		return "Connections at " + joinAnd(tight) + " look tight; allow more time or pick later outbounds.", confidence
	case len(tight) == 1:
		return "Connection at " + tight[0] + " looks tight; allow more time or pick a later outbound.", confidence
	case len(unknown) > 0:
		return "Connection risk at " + joinAnd(unknown) + " is unknown; allow extra time or choose a later outbound.", confidence
	default:
		return fmt.Sprintf("%s–%s itinerary connections look comfortable on historical data.", out.Summary.Origin, out.Summary.Destination), confidence
	}
}

func riskyConnections(connections []model.ItineraryConnection) (tight, unknown []string) {
	for _, conn := range connections {
		switch conn.Status {
		case model.ConnectionTight:
			tight = append(tight, conn.Airport)
		case model.ConnectionUnknown:
			unknown = append(unknown, conn.Airport)
		}
	}

	return tight, unknown
}

func mixedConnectionSummary(tight, unknown []string) string {
	tightClause := "Connection at " + joinAnd(tight) + " looks tight"
	if len(tight) > 1 {
		tightClause = "Connections at " + joinAnd(tight) + " look tight"
	}

	return tightClause + " and risk at " + joinAnd(unknown) + " is unknown; allow more time or pick a later outbound."
}

func joinAnd(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
	}
}
