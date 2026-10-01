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
	outlook := &model.RouteOutlook{RecommendedConnectionMinutes: testRecommended()}
	countries := map[string]string{
		testConnOrigin:   "US",
		testTravelOrigin: "US",
		testAirportLAX:   "US",
		testLHR:          "GB",
		testCDG:          "FR",
		"DEN":            "US",
	}

	dd, di, id, ii := 45, 60, 120, 180
	layover115 := 115
	slack70 := 70
	layover35 := 35
	slackNeg := -10
	layover540 := 540
	slack495 := 495
	layover180 := 180
	slack120 := 120

	tests := []struct {
		name          string
		inbound       model.ItineraryLeg
		outbound      model.ItineraryLeg
		outlook       *model.RouteOutlook
		countries     map[string]string
		wantStatus    string
		wantType      string
		wantLayover   *int
		wantRecommend *int
		wantSlack     *int
	}{
		{
			name:          "ok domestic",
			inbound:       testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound:      testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime1100, "1400"),
			outlook:       outlook,
			countries:     countries,
			wantStatus:    model.ConnectionOK,
			wantType:      model.ConnectionDomesticToDomestic,
			wantLayover:   &layover115,
			wantRecommend: &dd,
			wantSlack:     &slack70,
		},
		{
			name:          "tight domestic",
			inbound:       testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound:      testLeg(testTravelOrigin, testAirportLAX, testDateTue, "0940", "1200"),
			outlook:       outlook,
			countries:     countries,
			wantStatus:    model.ConnectionTight,
			wantType:      model.ConnectionDomesticToDomestic,
			wantLayover:   &layover35,
			wantRecommend: &dd,
			wantSlack:     &slackNeg,
		},
		{
			name:          "overnight",
			inbound:       testLeg(testConnOrigin, testTravelOrigin, testDateTue, "1800", "2200"),
			outbound:      testLeg(testTravelOrigin, testAirportLAX, testDateWed, testTime0700, "1000"),
			outlook:       outlook,
			countries:     countries,
			wantStatus:    model.ConnectionOK,
			wantType:      model.ConnectionDomesticToDomestic,
			wantLayover:   &layover540,
			wantRecommend: &dd,
			wantSlack:     &slack495,
		},
		{
			name:          "domestic to international",
			inbound:       testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0900),
			outbound:      testLeg(testTravelOrigin, testLHR, testDateTue, "1200", "2300"),
			outlook:       outlook,
			countries:     countries,
			wantStatus:    model.ConnectionOK,
			wantType:      model.ConnectionDomesticToInternational,
			wantLayover:   &layover180,
			wantRecommend: &di,
			wantSlack:     &slack120,
		},
		{
			name:          "international to domestic",
			inbound:       testLeg(testLHR, testTravelOrigin, testDateTue, testTime0700, testTime0900),
			outbound:      testLeg(testTravelOrigin, testAirportLAX, testDateTue, "1200", "1500"),
			outlook:       outlook,
			countries:     countries,
			wantStatus:    model.ConnectionOK,
			wantType:      model.ConnectionInternationalToDomestic,
			wantLayover:   &layover180,
			wantRecommend: &id,
			wantSlack:     intPtr(60),
		},
		{
			name:          "international to international",
			inbound:       testLeg(testTravelOrigin, testLHR, testDateTue, testTime0700, "1800"),
			outbound:      testLeg(testLHR, testCDG, testDateTue, "2100", "2300"),
			outlook:       outlook,
			countries:     countries,
			wantStatus:    model.ConnectionOK,
			wantType:      model.ConnectionInternationalToInternational,
			wantLayover:   intPtr(180),
			wantRecommend: &ii,
			wantSlack:     intPtr(0),
		},
		{
			name:        "mismatched airports",
			inbound:     testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0900),
			outbound:    testLeg("DEN", testAirportLAX, testDateTue, "1200", "1500"),
			outlook:     outlook,
			countries:   countries,
			wantStatus:  model.ConnectionUnknown,
			wantLayover: &layover180,
		},
		{
			name:        "missing country",
			inbound:     testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0900),
			outbound:    testLeg(testTravelOrigin, testAirportLAX, testDateTue, "1200", "1500"),
			outlook:     outlook,
			countries:   map[string]string{testConnOrigin: "US", testAirportLAX: "US"},
			wantStatus:  model.ConnectionUnknown,
			wantLayover: &layover180,
		},
		{
			name:        "missing outlook",
			inbound:     testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound:    testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime1100, "1400"),
			countries:   countries,
			wantStatus:  model.ConnectionUnknown,
			wantType:    model.ConnectionDomesticToDomestic,
			wantLayover: &layover115,
		},
		{
			name:     "null recommendation",
			inbound:  testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, testTime0905),
			outbound: testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime1100, "1400"),
			outlook: &model.RouteOutlook{RecommendedConnectionMinutes: model.RecommendedConnectionMinutes{
				DomesticToInternational: &di,
			}},
			countries:   countries,
			wantStatus:  model.ConnectionUnknown,
			wantType:    model.ConnectionDomesticToDomestic,
			wantLayover: &layover115,
		},
		{
			name:          "negative layover",
			inbound:       testLeg(testConnOrigin, testTravelOrigin, testDateTue, testTime0700, "1500"),
			outbound:      testLeg(testTravelOrigin, testAirportLAX, testDateTue, testTime0900, "1200"),
			outlook:       outlook,
			countries:     countries,
			wantStatus:    model.ConnectionUnknown,
			wantType:      model.ConnectionDomesticToDomestic,
			wantRecommend: &dd,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssessConnection(tt.inbound, tt.outbound, tt.outlook, tt.countries)
			if got.Status != tt.wantStatus {
				t.Errorf("status = %q, want %q", got.Status, tt.wantStatus)
			}

			assertStringPtr(t, "connection_type", got.ConnectionType, tt.wantType)
			assertIntPtr(t, "layover_minutes", got.LayoverMinutes, tt.wantLayover)
			assertIntPtr(t, "recommended_connection_minutes", got.RecommendedConnectionMinutes, tt.wantRecommend)
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
		testConnOrigin:   "US",
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

func testRecommended() model.RecommendedConnectionMinutes {
	dd, di, id, ii := 45, 60, 120, 180

	return model.RecommendedConnectionMinutes{
		DomesticToDomestic:           &dd,
		DomesticToInternational:      &di,
		InternationalToDomestic:      &id,
		InternationalToInternational: &ii,
	}
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
