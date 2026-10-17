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
			legIn.Carrier, legIn.FlightNumber, legIn.DepTime,
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

	summary, confidence, sampleSize, sampleReason := itineraryToolResult(out)

	return successResult(summary, confidence, sampleSize, sampleReason, &checkItineraryData{Itinerary: out})
}

func itineraryToolResult(out model.ItineraryOutlookResponse) (string, string, *int, string) {
	confidence := out.Summary.Confidence
	sampleSize, sampleReason := minLegOutlookSample(out.Legs)

	switch {
	case out.Summary.ConnectionRisk:
		hub, status := tightestConnection(out.Connections)
		if hub == "" {
			return "Treat connection timing as uncertain until every leg has enough history.", confidence, sampleSize, sampleReason
		}

		if status == model.ConnectionUnknown {
			return fmt.Sprintf("Connection risk at %s is unknown; allow extra time or choose a later outbound.", hub), confidence, sampleSize, sampleReason
		}

		return fmt.Sprintf("Connection at %s looks %s; allow more time or pick a later outbound.", hub, status), confidence, sampleSize, sampleReason
	default:
		return fmt.Sprintf("%s–%s itinerary connections look comfortable on historical data.", out.Summary.Origin, out.Summary.Destination), confidence, sampleSize, sampleReason
	}
}

func tightestConnection(connections []model.ItineraryConnection) (hub string, status string) {
	worstRank := -1

	for _, conn := range connections {
		rank := connectionStatusRank(conn.Status)
		if rank > worstRank {
			worstRank = rank
			hub = conn.Airport
			status = conn.Status
		}
	}

	return hub, status
}

func connectionStatusRank(status string) int {
	switch status {
	case model.ConnectionTight:
		return 3
	case model.ConnectionUnknown:
		return 2
	case model.ConnectionOK:
		return 1
	default:
		return 0
	}
}

func minLegOutlookSample(legs []model.ItineraryOutlookLeg) (*int, string) {
	var (
		size   *int
		reason string
	)

	for _, leg := range legs {
		if leg.Outlook == nil {
			continue
		}

		sample := leg.Outlook.Analysis.SampleSize
		if size == nil || sample < *size {
			size = &sample
			reason = leg.Outlook.Analysis.SampleReason
		}
	}

	return size, reason
}
