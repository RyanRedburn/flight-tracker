package store

import (
	"strconv"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	testDateTue    = "2026-10-06"
	testDateWed    = "2026-10-07"
	testDateThu    = "2026-10-08"
	testTime0700   = "0700"
	testTime0900   = "0900"
	testTime0905   = "0905"
	testTime1100   = "1100"
	testConnOrigin = "PHL"
	testAirportBOS = "BOS"
	testLHR        = "LHR"
	testCDG        = "CDG"
)

func TestLayoverMinutes(t *testing.T) {
	tests := []struct {
		name    string
		inDate  string
		inArr   string
		outDate string
		outDep  string
		want    int
		wantOK  bool
	}{
		{name: "same day", inDate: testDateTue, inArr: testTime0905, outDate: testDateTue, outDep: testTime1100, want: 115, wantOK: true},
		{name: "overnight", inDate: testDateTue, inArr: "2200", outDate: testDateWed, outDep: testTime0700, want: 540, wantOK: true},
		{name: "multi-day", inDate: testDateTue, inArr: "2200", outDate: testDateThu, outDep: testTime0700, want: 1980, wantOK: true},
		{name: "zero", inDate: testDateTue, inArr: testTime0900, outDate: testDateTue, outDep: testTime0900, want: 0, wantOK: true},
		{name: "negative", inDate: testDateTue, inArr: testTime1100, outDate: testDateTue, outDep: testTime0900, wantOK: false},
		{name: "invalid time", inDate: testDateTue, inArr: "2500", outDate: testDateTue, outDep: testTime1100, wantOK: false},
		{name: "invalid date", inDate: "2026-13-01", inArr: testTime0900, outDate: testDateTue, outDep: testTime1100, wantOK: false},
		{name: "unpadded", inDate: testDateTue, inArr: "905", outDate: testDateTue, outDep: testTime1100, want: 115, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := LayoverMinutes(tt.inDate, tt.inArr, tt.outDate, tt.outDep)
			if ok != tt.wantOK || (ok && got != tt.want) {
				t.Fatalf("LayoverMinutes() = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestConnectionType(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		hub    string
		dest   string
		want   string
		wantOK bool
	}{
		{name: "domestic", origin: "US", hub: "US", dest: "US", want: model.ConnectionDomesticToDomestic, wantOK: true},
		{name: "case insensitive", origin: "us", hub: "US", dest: "us", want: model.ConnectionDomesticToDomestic, wantOK: true},
		{name: "domestic to international", origin: "US", hub: "US", dest: "GB", want: model.ConnectionDomesticToInternational, wantOK: true},
		{name: "international to domestic", origin: "GB", hub: "US", dest: "US", want: model.ConnectionInternationalToDomestic, wantOK: true},
		{name: "international to international", origin: "US", hub: "GB", dest: "FR", want: model.ConnectionInternationalToInternational, wantOK: true},
		{name: "same foreign ends", origin: "US", hub: "GB", dest: "US", want: model.ConnectionInternationalToInternational, wantOK: true},
		{name: "missing hub", origin: "US", hub: " ", dest: "US", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ConnectionType(tt.origin, tt.hub, tt.dest)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("ConnectionType() = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestAssessConnection(t *testing.T) {
	outlook := &model.RouteOutlook{
		SampleSize:   40,
		SampleReason: model.SampleReasonSufficient,
		Confidence:   model.ConfidenceHigh,
		Connection:   testRecommended(),
	}
	floorGuidance := testRecommended()
	floorGuidance.DomesticToDomestic.FloorOnly = true
	floorOutlook := &model.RouteOutlook{
		SampleSize:   40,
		SampleReason: model.SampleReasonSufficient,
		Confidence:   model.ConfidenceHigh,
		Connection:   floorGuidance,
	}
	airportFloor := false
	staticFloor := true
	countries := map[string]string{
		testConnOrigin:   "US",
		testTravelOrigin: "US",
		testAirportLAX:   "US",
		testLHR:          "GB",
		testCDG:          "FR",
		"DEN":            "US",
	}

	dd, di, id, ii := 45, 60, 120, 180
	looseDD := dd + model.ConnectionLooseSlackMinutes
	looseDI := di + model.ConnectionLooseSlackMinutes
	looseID := id + model.ConnectionLooseSlackMinutes
	looseII := ii + model.ConnectionLooseSlackMinutes
	layover115 := 115
	slack70 := 70
	layover60 := 60
	slack15 := 15
	layover35 := 35
	slackNeg := -10
	layover540 := 540
	slack495 := 495
	layover180 := 180
	slack120 := 120
	layover1980 := 1980
	slack1935 := 1935

	tests := []struct {
		name           string
		inbound        model.ItineraryLeg
		outbound       model.ItineraryLeg
		outlook        *model.RouteOutlook
		countries      map[string]string
		wantStatus     string
		wantConfidence string
		wantReason     string
		wantOvernight  bool
		wantFloorOnly  *bool
		wantType       string
		wantLayover    *int
		wantRecommend  *int
		wantLoose      *int
		wantSlack      *int
	}{
		{
			name:           "loose domestic",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound:       testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime1100, "1400"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionLoose,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonEvaluated,
			wantFloorOnly:  &airportFloor,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover115,
			wantRecommend:  &dd,
			wantLoose:      &looseDD,
			wantSlack:      &slack70,
		},
		{
			name:           "ok domestic",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound:       testLeg(testTravelOrigin, testAirportLAX, testDateTue, "1005", "1200"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionOK,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonEvaluated,
			wantFloorOnly:  &airportFloor,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover60,
			wantRecommend:  &dd,
			wantLoose:      &looseDD,
			wantSlack:      &slack15,
		},
		{
			name:           "tight domestic",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound:       testLeg(testTravelOrigin, testAirportLAX, testDateTue, "0940", "1200"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionTight,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonEvaluated,
			wantFloorOnly:  &airportFloor,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover35,
			wantRecommend:  &dd,
			wantLoose:      &looseDD,
			wantSlack:      &slackNeg,
		},
		{
			name:           "overnight",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, "1800", "2200"),
			outbound:       testLeg(testTravelOrigin, testAirportLAX, testDateWed, testTime0700, "1000"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionLoose,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonOvernight,
			wantOvernight:  true,
			wantFloorOnly:  &airportFloor,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover540,
			wantRecommend:  &dd,
			wantLoose:      &looseDD,
			wantSlack:      &slack495,
		},
		{
			name:           "domestic to international",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0900),
			outbound:       testLeg(testTravelOrigin, testLHR, testDateTue, "1200", "2300"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionLoose,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonEvaluated,
			wantFloorOnly:  &airportFloor,
			wantType:       model.ConnectionDomesticToInternational,
			wantLayover:    &layover180,
			wantRecommend:  &di,
			wantLoose:      &looseDI,
			wantSlack:      &slack120,
		},
		{
			name:           "international to domestic",
			inbound:        testLeg(testLHR, testTravelOrigin, testDateTue, testTime0700, testTime0900),
			outbound:       testLeg(testTravelOrigin, testAirportLAX, testDateTue, "1200", "1500"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionLoose,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonEvaluated,
			wantFloorOnly:  &airportFloor,
			wantType:       model.ConnectionInternationalToDomestic,
			wantLayover:    &layover180,
			wantRecommend:  &id,
			wantLoose:      &looseID,
			wantSlack:      intPtr(60),
		},
		{
			name:           "international to international",
			inbound:        testLeg(testTravelOrigin, testLHR, testDateTue, testTime0700, "1800"),
			outbound:       testLeg(testLHR, testCDG, testDateTue, "2100", "2300"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionOK,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonEvaluated,
			wantFloorOnly:  &airportFloor,
			wantType:       model.ConnectionInternationalToInternational,
			wantLayover:    intPtr(180),
			wantRecommend:  &ii,
			wantLoose:      &looseII,
			wantSlack:      intPtr(0),
		},
		{
			name:           "mismatched airports",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0900),
			outbound:       testLeg("DEN", testAirportLAX, testDateTue, "1200", "1500"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionUnknown,
			wantConfidence: model.ConfidenceUnknown,
			wantReason:     model.ConnectionReasonMultiAirport,
			wantLayover:    &layover180,
		},
		{
			name:           "missing country",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0900),
			outbound:       testLeg(testTravelOrigin, testAirportLAX, testDateTue, "1200", "1500"),
			outlook:        outlook,
			countries:      map[string]string{testConnOrigin: "US", testAirportLAX: "US"},
			wantStatus:     model.ConnectionUnknown,
			wantConfidence: model.ConfidenceUnknown,
			wantReason:     model.ConnectionReasonMissingCountry,
			wantLayover:    &layover180,
		},
		{
			name:           "missing outlook",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound:       testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime1100, "1400"),
			countries:      countries,
			wantStatus:     model.ConnectionUnknown,
			wantConfidence: model.ConfidenceUnknown,
			wantReason:     model.ConnectionReasonMissingOutlook,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover115,
		},
		{
			name:     "null recommendation",
			inbound:  testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound: testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime1100, "1400"),
			outlook: &model.RouteOutlook{
				SampleSize:   40,
				SampleReason: model.SampleReasonSufficient,
				Confidence:   model.ConfidenceHigh,
				Connection: model.ConnectionGuidance{
					DomesticToInternational: model.NewConnectionMinutes(di),
				},
			},
			countries:      countries,
			wantStatus:     model.ConnectionUnknown,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonMissingThreshold,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover115,
		},
		{
			name:           "negative layover",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, "1500"),
			outbound:       testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime0900, "1200"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionUnknown,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonNegativeLayover,
			wantFloorOnly:  &airportFloor,
			wantType:       model.ConnectionDomesticToDomestic,
			wantRecommend:  &dd,
			wantLoose:      &looseDD,
		},
		{
			name:           "floor only domestic",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound:       testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime1100, "1400"),
			outlook:        floorOutlook,
			countries:      countries,
			wantStatus:     model.ConnectionLoose,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonEvaluated,
			wantFloorOnly:  &staticFloor,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover115,
			wantRecommend:  &dd,
			wantLoose:      &looseDD,
			wantSlack:      &slack70,
		},
		{
			name:           "multi-day overnight",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, "1800", "2200"),
			outbound:       testLeg(testTravelOrigin, testAirportLAX, testDateThu, testTime0700, "1000"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionLoose,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonOvernight,
			wantOvernight:  true,
			wantFloorOnly:  &airportFloor,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover1980,
			wantRecommend:  &dd,
			wantLoose:      &looseDD,
			wantSlack:      &slack1935,
		},
		{
			name:           "multi-airport overnight",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, "1800", "2200"),
			outbound:       testLeg("DEN", testAirportLAX, testDateWed, testTime0700, "1000"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionUnknown,
			wantConfidence: model.ConfidenceUnknown,
			wantReason:     model.ConnectionReasonMultiAirport,
			wantOvernight:  true,
			wantLayover:    &layover540,
		},
		{
			name:     "empty sample",
			inbound:  testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound: testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime1100, "1400"),
			outlook: &model.RouteOutlook{
				SampleReason: model.SampleReasonEmptySample,
				Confidence:   model.ConfidenceUnknown,
				Connection:   testRecommended(),
			},
			countries:      countries,
			wantStatus:     model.ConnectionUnknown,
			wantConfidence: model.ConfidenceUnknown,
			wantReason:     model.SampleReasonEmptySample,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover115,
		},
		{
			name:     "insufficient sample",
			inbound:  testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound: testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime1100, "1400"),
			outlook: &model.RouteOutlook{
				SampleSize:         4,
				InsufficientSample: true,
				SampleReason:       model.SampleReasonInsufficientSample,
				Confidence:         model.ConfidenceLow,
			},
			countries:      countries,
			wantStatus:     model.ConnectionUnknown,
			wantConfidence: model.ConfidenceLow,
			wantReason:     model.SampleReasonInsufficientSample,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover115,
		},
		{
			name:     "derived thin sample",
			inbound:  testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound: testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime1100, "1400"),
			outlook: &model.RouteOutlook{
				SampleSize:         4,
				InsufficientSample: true,
				Confidence:         model.ConfidenceLow,
			},
			countries:      countries,
			wantStatus:     model.ConnectionUnknown,
			wantConfidence: model.ConfidenceLow,
			wantReason:     model.SampleReasonInsufficientSample,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover115,
		},
		{
			name:     "overnight thin sample",
			inbound:  testLeg(testConnOrigin, testTravelOrigin, testDateTue, "1800", "2200"),
			outbound: testLeg(testTravelOrigin, testAirportLAX, testDateWed, testTime0700, "1000"),
			outlook: &model.RouteOutlook{
				SampleSize:         4,
				InsufficientSample: true,
				SampleReason:       model.SampleReasonInsufficientSample,
				Confidence:         model.ConfidenceLow,
			},
			countries:      countries,
			wantStatus:     model.ConnectionUnknown,
			wantConfidence: model.ConfidenceLow,
			wantReason:     model.SampleReasonInsufficientSample,
			wantOvernight:  true,
			wantType:       model.ConnectionDomesticToDomestic,
			wantLayover:    &layover540,
		},
		{
			name:           "invalid schedule",
			inbound:        testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, "2500"),
			outbound:       testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime1100, "1400"),
			outlook:        outlook,
			countries:      countries,
			wantStatus:     model.ConnectionUnknown,
			wantConfidence: model.ConfidenceHigh,
			wantReason:     model.ConnectionReasonInvalidSchedule,
			wantFloorOnly:  &airportFloor,
			wantType:       model.ConnectionDomesticToDomestic,
			wantRecommend:  &dd,
			wantLoose:      &looseDD,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssessConnection(tt.inbound, tt.outbound, tt.outlook, tt.countries)
			if got.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", got.Status, tt.wantStatus)
			}

			if got.Confidence != tt.wantConfidence {
				t.Errorf("confidence = %q, want %q", got.Confidence, tt.wantConfidence)
			}

			if got.Reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", got.Reason, tt.wantReason)
			}

			if got.Overnight != tt.wantOvernight {
				t.Errorf("overnight = %v, want %v", got.Overnight, tt.wantOvernight)
			}

			assertBoolPtr(t, "floor_only", got.FloorOnly, tt.wantFloorOnly)
			assertStringPtr(t, "connection_type", got.ConnectionType, tt.wantType)
			assertIntPtr(t, "layover_minutes", got.LayoverMinutes, tt.wantLayover)
			assertIntPtr(t, "recommended_minutes", got.RecommendedMinutes, tt.wantRecommend)
			assertIntPtr(t, "loose_minutes", got.LooseMinutes, tt.wantLoose)
			assertIntPtr(t, "slack_minutes", got.SlackMinutes, tt.wantSlack)
		})
	}
}

func TestBuildItineraryOutlookMissingLeg(t *testing.T) {
	window := 45
	legs := []model.ItineraryLeg{
		testLeg("bos", testTravelOrigin, testDateTue, "700", "905"),
		{
			Origin:               testTravelOrigin,
			Dest:                 testAirportLAX,
			Carrier:              "UA",
			Date:                 testDateTue,
			DepTime:              testTime1100,
			ArrTime:              "1400",
			DepTimeWindowMinutes: &window,
		},
	}
	outlook := &model.RouteOutlook{Origin: testTravelOrigin, Dest: testAirportLAX}

	got := BuildItineraryOutlook(legs, []*model.RouteOutlook{nil, outlook}, []string{"route outlook not found", ""}, map[string]string{
		testAirportBOS:   "US",
		testTravelOrigin: "US",
		testAirportLAX:   "US",
	})

	if len(got.Legs) != 2 || len(got.Connections) != 1 {
		t.Fatalf("legs=%d connections=%d", len(got.Legs), len(got.Connections))
	}

	if got.Legs[0].Index != 0 || got.Legs[0].DayOfWeek != 2 || got.Legs[0].Outlook != nil {
		t.Fatalf("first leg = %+v", got.Legs[0])
	}

	if got.Legs[0].Error == nil || *got.Legs[0].Error != "route outlook not found" {
		t.Fatalf("error = %v", got.Legs[0].Error)
	}

	if got.Legs[0].DepTime != testTime0700 || got.Legs[0].ArrTime != testTime0905 || got.Legs[0].DepTimeWindowMinutes != DefaultDepTimeWindowMinutes {
		t.Fatalf("echo = %+v", got.Legs[0])
	}

	if got.Legs[1].Error != nil || got.Legs[1].Outlook != outlook || got.Legs[1].DepTimeWindowMinutes != 45 {
		t.Fatalf("second leg = %+v", got.Legs[1])
	}

	if got.Connections[0].AfterLeg != 0 || got.Connections[0].Airport != testTravelOrigin || got.Connections[0].Status != model.ConnectionUnknown {
		t.Fatalf("connection = %+v", got.Connections[0])
	}

	if got.Connections[0].Reason != model.ConnectionReasonMissingOutlook || got.Connections[0].Confidence != model.ConfidenceUnknown {
		t.Fatalf("connection reason = %q confidence %q", got.Connections[0].Reason, got.Connections[0].Confidence)
	}
}

func testLeg(origin, dest, date, dep, arr string) model.ItineraryLeg {
	return model.ItineraryLeg{
		Origin:  origin,
		Dest:    dest,
		Carrier: "UA",
		Date:    date,
		DepTime: dep,
		ArrTime: arr,
	}
}

func testRecommended() model.ConnectionGuidance {
	return model.ConnectionGuidanceFromRecommended(45, 60, 120, 180)
}

func assertStringPtr(t *testing.T, name string, got *string, want string) {
	t.Helper()

	if want == "" {
		if got != nil {
			t.Errorf("%s = %q, want null", name, *got)
		}

		return
	}

	if got == nil || *got != want {
		t.Errorf("%s = %v, want %q", name, got, want)
	}
}

func assertBoolPtr(t *testing.T, name string, got, want *bool) {
	t.Helper()

	switch {
	case want == nil && got == nil:
		return
	case want == nil || got == nil || *want != *got:
		t.Errorf("%s = %v, want %v", name, formatBoolPtr(got), formatBoolPtr(want))
	}
}

func formatBoolPtr(v *bool) string {
	if v == nil {
		return "null"
	}

	if *v {
		return "true"
	}

	return "false"
}

func assertIntPtr(t *testing.T, name string, got, want *int) {
	t.Helper()

	switch {
	case want == nil && got == nil:
		return
	case want == nil || got == nil || *want != *got:
		t.Errorf("%s = %v, want %v", name, formatIntPtr(got), formatIntPtr(want))
	}
}

func formatIntPtr(v *int) string {
	if v == nil {
		return "null"
	}

	return strconv.Itoa(*v)
}
