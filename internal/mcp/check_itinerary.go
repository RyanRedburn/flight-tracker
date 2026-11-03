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
	FlightNumber         string `json:"flight_number,omitempty" jsonschema:"Marketing flight number when origin and destination are unknown. 1 to 4 digits; leading zeros are ignored"`
	Date                 string `json:"date" jsonschema:"Travel date (YYYY-MM-DD)"`
	DepTime              string `json:"dep_time" jsonschema:"Scheduled departure (hhmm)"`
	ArrTime              string `json:"arr_time" jsonschema:"Scheduled arrival (hhmm)"`
	DepTimeWindowMinutes *int   `json:"dep_time_window_minutes,omitempty" jsonschema:"Minutes around dep_time for outlook history (1–120)"`
}

type checkItineraryInput struct {
	Legs []checkItineraryLegInput `json:"legs" jsonschema:"Ordered flight legs, 2 to 4"`
}

type checkItineraryData struct {
	Legs        []model.ItineraryOutlookLeg `json:"legs"`
	Connections []model.ItineraryConnection `json:"connections"`
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
	legProps["flight_number"].MaxLength = jsonschema.Ptr(store.FlightNumberRawMaxLen)
	legProps["flight_number"].Pattern = store.FlightNumberPattern
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
	fromFlightNumber := make([]bool, len(in.Legs))
	needLookup := false

	for i, legIn := range in.Legs {
		leg, lookup, fail := prepareItineraryLeg(legIn)
		if fail != nil {
			return failureResult[checkItineraryData](fail)
		}

		legs[i] = leg

		fromFlightNumber[i] = lookup
		if lookup {
			needLookup = true
		}
	}

	if !needLookup {
		if fail := validateItineraryLegs(legs); fail != nil {
			return failureResult[checkItineraryData](fail)
		}
	}

	for i, legIn := range in.Legs {
		if !fromFlightNumber[i] {
			continue
		}

		window := store.ResolveDepTimeWindowMinutes(legIn.DepTimeWindowMinutes)

		origin, dest, _, fail := resolveRouteFromFlightNumber(ctx, s, logger, toolCheckItinerary,
			legIn.Carrier, legIn.FlightNumber, []string{legIn.DepTime}, legIn.Origin, legIn.Dest, window)
		if fail != nil {
			return failureResult[checkItineraryData](fail)
		}

		legs[i].Origin = origin
		legs[i].Dest = dest
	}

	if needLookup {
		if fail := validateItineraryLegs(legs); fail != nil {
			return failureResult[checkItineraryData](fail)
		}
	}

	out, err := store.ItineraryOutlook(ctx, s, legs)
	if err != nil {
		logger.Error(toolCheckItinerary, "error", err)

		return failureResult[checkItineraryData](&Failure{
			Code:    CodeUnavailable,
			Message: "Itinerary outlook is unavailable. Try again.",
		})
	}

	if fail := missingItineraryAirports(ctx, s, logger, legs, fromFlightNumber, out); fail != nil {
		return failureResult[checkItineraryData](fail)
	}

	summary, confidence := itineraryToolResult(out)

	return successResult(summary, confidence, &checkItineraryData{
		Legs:        out.Legs,
		Connections: out.Connections,
	})
}

func prepareItineraryLeg(in checkItineraryLegInput) (model.ItineraryLeg, bool, *Failure) {
	carrier := normalizeCarrier(in.Carrier)
	if len(carrier) != 2 {
		return model.ItineraryLeg{}, false, &Failure{Code: CodeInvalidInput, Message: msgCarrierCode}
	}

	if in.DepTimeWindowMinutes != nil {
		if err := store.DepTimeWindowError(*in.DepTimeWindowMinutes); err != nil {
			return model.ItineraryLeg{}, false, invalidInput(err)
		}
	}

	if _, ok := store.ISOWeekdayFromDate(in.Date); !ok {
		return model.ItineraryLeg{}, false, invalidInput(store.ErrInvalidDate)
	}

	if _, ok := store.HHMMToMinutes(in.DepTime); !ok {
		return model.ItineraryLeg{}, false, invalidInput(store.ErrInvalidHHMM)
	}

	if _, ok := store.HHMMToMinutes(in.ArrTime); !ok {
		return model.ItineraryLeg{}, false, &Failure{
			Code:    CodeInvalidInput,
			Message: "arr_time must be a valid local time (hhmm).",
		}
	}

	origin, dest, supplied, fail := normalizeSuppliedRoute(in.Origin, in.Dest)
	if fail != nil {
		return model.ItineraryLeg{}, false, fail
	}

	if !supplied {
		if _, err := store.NormalizeFlightNumber(in.FlightNumber); err != nil {
			return model.ItineraryLeg{}, false, &Failure{
				Code:    CodeInvalidInput,
				Message: "Flight number is required and must be 1 to 4 digits.",
			}
		}
	}

	return model.ItineraryLeg{
		Origin:               origin,
		Dest:                 dest,
		Carrier:              carrier,
		Date:                 strings.TrimSpace(in.Date),
		DepTime:              strings.TrimSpace(in.DepTime),
		ArrTime:              strings.TrimSpace(in.ArrTime),
		DepTimeWindowMinutes: in.DepTimeWindowMinutes,
	}, !supplied, nil
}

func validateItineraryLegs(legs []model.ItineraryLeg) *Failure {
	req := model.ItineraryOutlookRequest{Legs: legs}
	if err := req.Validate(); err != nil {
		return &Failure{Code: CodeInvalidInput, Message: err.Error()}
	}

	return nil
}

func missingItineraryAirports(ctx context.Context, s store.Store, logger *slog.Logger, legs []model.ItineraryLeg, fromFlightNumber []bool, out model.ItineraryOutlookResponse) *Failure {
	for i, leg := range out.Legs {
		if leg.Error == nil || *leg.Error != store.ItineraryLegOutlookNotFound {
			continue
		}

		if fromFlightNumber[i] {
			continue
		}

		fail := explainMissingRoute(ctx, s, logger, toolCheckItinerary, legs[i].Carrier, legs[i].Origin, legs[i].Dest, false)
		if fail.Code == CodeRouteNotFound {
			continue
		}

		return fail
	}

	return nil
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
