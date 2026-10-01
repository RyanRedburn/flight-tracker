package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestGoldenOutlookItineraryConnection(t *testing.T) {
	outlook := &model.RouteOutlook{
		Origin:       testOriginBOS,
		Dest:         testOriginORD,
		Carrier:      "UA",
		DayOfWeek:    2,
		DepTime:      "0700",
		SampleSize:   40,
		SampleReason: model.SampleReasonSufficient,
		Confidence:   model.ConfidenceHigh,
		Connection:   model.ConnectionGuidanceFromRecommended(60, 75, 90, 120),
	}
	countries := map[string]string{
		testOriginBOS: "US",
		testOriginORD: "US",
		testDestLAX:   "US",
	}

	gotOutlook := getOutlook(t, outlook)
	bucket := gotOutlook.Connection.DomesticToDomestic

	if bucket == nil || bucket.RecommendedMinutes != 60 || bucket.LooseMinutes != 90 {
		t.Fatalf("outlook domestic_to_domestic = %+v", bucket)
	}

	if gotOutlook.Confidence != model.ConfidenceHigh {
		t.Fatalf("outlook confidence = %q", gotOutlook.Confidence)
	}

	conn := postConnection(t, outlook, countries, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
		{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "0700", ArrTime: testArr0905},
		{Origin: testOriginORD, Dest: testDestLAX, Carrier: "UA", Date: testDateTue, DepTime: "1100", ArrTime: "1330"},
	}})

	if conn.LayoverMinutes == nil {
		t.Fatal("missing layover")
	}

	wantStatus := model.ConnectionStatus(*conn.LayoverMinutes, bucket)
	if conn.Status != wantStatus || conn.Status != model.ConnectionLoose {
		t.Fatalf("status = %q, outlook thresholds imply %q", conn.Status, wantStatus)
	}

	if conn.RecommendedMinutes == nil || *conn.RecommendedMinutes != bucket.RecommendedMinutes {
		t.Fatalf("recommended_minutes = %v, outlook = %d", conn.RecommendedMinutes, bucket.RecommendedMinutes)
	}

	if conn.LooseMinutes == nil || *conn.LooseMinutes != bucket.LooseMinutes {
		t.Fatalf("loose_minutes = %v, outlook = %d", conn.LooseMinutes, bucket.LooseMinutes)
	}

	if conn.Confidence != gotOutlook.Confidence {
		t.Fatalf("itinerary confidence = %q, outlook = %q", conn.Confidence, gotOutlook.Confidence)
	}
}

func TestGoldenThinSampleStaysUnknown(t *testing.T) {
	outlook := &model.RouteOutlook{
		Origin:             testOriginBOS,
		Dest:               testOriginORD,
		Carrier:            "UA",
		SampleSize:         4,
		InsufficientSample: true,
		SampleReason:       model.SampleReasonInsufficientSample,
		Confidence:         model.ConfidenceLow,
	}
	countries := map[string]string{
		testOriginBOS: "US",
		testOriginORD: "US",
		testDestLAX:   "US",
	}

	gotOutlook := getOutlook(t, outlook)
	if gotOutlook.Confidence != model.ConfidenceLow || gotOutlook.Connection.DomesticToDomestic != nil {
		t.Fatalf("thin outlook = %+v", gotOutlook.Connection)
	}

	conn := postConnection(t, outlook, countries, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
		{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "0700", ArrTime: testArr0905},
		{Origin: testOriginORD, Dest: testDestLAX, Carrier: "UA", Date: testDateTue, DepTime: "1100", ArrTime: "1330"},
	}})

	if conn.Status != model.ConnectionUnknown || conn.Confidence != gotOutlook.Confidence {
		t.Fatalf("connection = %+v, outlook confidence %q", conn, gotOutlook.Confidence)
	}

	if conn.RecommendedMinutes != nil || conn.LooseMinutes != nil || conn.SlackMinutes != nil {
		t.Fatalf("thin connection published thresholds: %+v", conn)
	}
}

func TestGoldenItineraryConfidenceUnknownWhenAirportsDiffer(t *testing.T) {
	outlook := &model.RouteOutlook{
		Origin:       testOriginBOS,
		Dest:         testOriginORD,
		Carrier:      "UA",
		SampleSize:   40,
		SampleReason: model.SampleReasonSufficient,
		Confidence:   model.ConfidenceHigh,
		Connection:   model.ConnectionGuidanceFromRecommended(60, 75, 90, 120),
	}
	countries := map[string]string{
		testOriginBOS:  "US",
		testOriginORD:  "US",
		testAirportDEN: "US",
		testDestLAX:    "US",
	}

	gotOutlook := getOutlook(t, outlook)
	conn := postConnection(t, outlook, countries, model.ItineraryOutlookRequest{Legs: []model.ItineraryLeg{
		{Origin: testOriginBOS, Dest: testOriginORD, Carrier: "UA", Date: testDateTue, DepTime: "0700", ArrTime: testArr0900},
		{Origin: testAirportDEN, Dest: testDestLAX, Carrier: "UA", Date: testDateTue, DepTime: "1200", ArrTime: "1500"},
	}})

	// The inbound sample is usable, but this hop is not a same-airport connection,
	// so the connection does not inherit that confidence or a status.
	if gotOutlook.Confidence != model.ConfidenceHigh {
		t.Fatalf("outlook confidence = %q", gotOutlook.Confidence)
	}

	if conn.Status != model.ConnectionUnknown || conn.Confidence != model.ConfidenceUnknown || conn.ConnectionType != nil {
		t.Fatalf("connection = %+v", conn)
	}
}

func getOutlook(t *testing.T, outlook *model.RouteOutlook) model.RouteOutlook {
	t.Helper()

	h := NewRoutesHandler(&storetest.Stub{
		RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
			return outlook, nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/routes/outlook?origin=BOS&dest=ORD&carrier=UA&day_of_week=2&dep_time=0700", nil)
	rec := httptest.NewRecorder()
	h.Outlook(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("outlook status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var got model.RouteOutlook
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode outlook: %v", err)
	}

	return got
}

func postConnection(t *testing.T, outlook *model.RouteOutlook, countries map[string]string, body model.ItineraryOutlookRequest) model.ItineraryConnection {
	t.Helper()

	rec := postItinerary(t, itineraryHandler(outlook, countries, nil), body)
	out := decodeItinerary(t, rec)

	if len(out.Connections) != 1 || out.Legs[0].Outlook == nil {
		t.Fatalf("itinerary = %+v", out)
	}

	if out.Legs[0].Outlook.Confidence != outlook.Confidence {
		t.Fatalf("embedded outlook confidence = %q, want %q", out.Legs[0].Outlook.Confidence, outlook.Confidence)
	}

	return out.Connections[0]
}
