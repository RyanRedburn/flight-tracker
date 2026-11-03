package store

import (
	"encoding/json"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

func TestAssembleRouteOutlookWeatherContrast(t *testing.T) {
	t.Parallel()

	// One route with weather history: origin fair 0.80 vs adverse 0.30, destination thin.
	got := AssembleRouteOutlookWeather([]WeatherCategoryCount{
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryVFRFair, OnTimeCount: 10, Flights: 20},
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryVFRFair, OnTimeCount: 30, Flights: 30},
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryThunder, OnTimeCount: 1, Flights: 10},
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryLIFR, OnTimeCount: 0, Flights: 5},
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryIFR, OnTimeCount: 4, Flights: 10},
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryMVFR, OnTimeCount: 2, Flights: 10},
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryWindy, OnTimeCount: 5, Flights: 10},
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryPrecip, OnTimeCount: 3, Flights: 5},
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryUnknown, OnTimeCount: 8, Flights: 10},
		{Side: WeatherSideOrigin, Category: WeatherUnmatched, OnTimeCount: 1, Flights: 8},
		{Side: WeatherSideOrigin, Category: "FOG", OnTimeCount: 1, Flights: 5},
		{Side: "enroute", Category: model.WeatherCategoryVFRFair, OnTimeCount: 9, Flights: 10},
		{Side: WeatherSideDest, Category: model.WeatherCategoryVFRFair, OnTimeCount: 3, Flights: 4},
		{Side: WeatherSideDest, Category: model.WeatherCategoryLIFR, OnTimeCount: 1, Flights: 3},
		{Side: WeatherSideDest, Category: model.WeatherCategoryUnknown, OnTimeCount: 2, Flights: 2},
	})

	assertOutlookWeatherSide(t, "origin", got.Origin, floatPtr(0.8), floatPtr(0.3), floatPtr(-0.5), 50, model.ConfidenceHigh)
	assertOutlookWeatherSide(t, "dest", got.Dest, floatPtr(0.75), floatPtr(0.33), floatPtr(-0.42), 3, model.ConfidenceLow)
}

func TestAssembleRouteOutlookWeatherEmptyAndThin(t *testing.T) {
	t.Parallel()

	empty := AssembleRouteOutlookWeather(nil)
	assertOutlookWeatherSide(t, "empty origin", empty.Origin, nil, nil, nil, 0, model.ConfidenceUnknown)
	assertOutlookWeatherSide(t, "empty dest", empty.Dest, nil, nil, nil, 0, model.ConfidenceUnknown)

	body, err := json.Marshal(empty.Origin)
	if err != nil {
		t.Fatalf("marshal empty side: %v", err)
	}

	for _, field := range []string{"fair_on_time_rate", "adverse_on_time_rate", "adverse_delta"} {
		if !jsonNullField(t, body, field) {
			t.Errorf("%s = %s, want null", field, body)
		}
	}

	unknownOnly := AssembleRouteOutlookWeather([]WeatherCategoryCount{
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryUnknown, OnTimeCount: 1, Flights: 4},
		{Side: WeatherSideOrigin, Category: WeatherUnmatched, OnTimeCount: 0, Flights: 6},
		{Side: WeatherSideDest, Category: model.WeatherCategoryVFRFair, OnTimeCount: 0, Flights: 0},
	})
	assertOutlookWeatherSide(t, "unknown only", unknownOnly.Origin, nil, nil, nil, 0, model.ConfidenceUnknown)
	assertOutlookWeatherSide(t, "zero fair row", unknownOnly.Dest, nil, nil, nil, 0, model.ConfidenceUnknown)

	fairOnly := AssembleRouteOutlookWeather([]WeatherCategoryCount{
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryVFRFair, OnTimeCount: 18, Flights: 20},
	})
	assertOutlookWeatherSide(t, "fair only", fairOnly.Origin, floatPtr(0.9), nil, nil, 0, model.ConfidenceUnknown)

	thin := AssembleRouteOutlookWeather([]WeatherCategoryCount{
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryVFRFair, OnTimeCount: 2, Flights: 3},
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryPrecip, OnTimeCount: 4, Flights: 9},
	})
	assertOutlookWeatherSide(t, "thin", thin.Origin, floatPtr(0.67), floatPtr(0.44), floatPtr(-0.23), 9, model.ConfidenceLow)

	below := AssembleRouteOutlookWeather([]WeatherCategoryCount{
		{Side: WeatherSideDest, Category: model.WeatherCategoryVFRFair, OnTimeCount: 10, Flights: 10},
		{Side: WeatherSideDest, Category: model.WeatherCategoryThunder, OnTimeCount: 8, Flights: 9},
	})
	assertOutlookWeatherSide(t, "one below minimum", below.Dest, floatPtr(1), floatPtr(0.89), floatPtr(-0.11), 9, model.ConfidenceLow)

	meets := AssembleRouteOutlookWeather([]WeatherCategoryCount{
		{Side: WeatherSideDest, Category: model.WeatherCategoryVFRFair, OnTimeCount: 10, Flights: 10},
		{Side: WeatherSideDest, Category: model.WeatherCategoryIFR, OnTimeCount: 10, Flights: 10},
	})
	assertOutlookWeatherSide(t, "ten flights", meets.Dest, floatPtr(1), floatPtr(1), floatPtr(0), 10, model.ConfidenceLow)

	better := AssembleRouteOutlookWeather([]WeatherCategoryCount{
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryVFRFair, OnTimeCount: 10, Flights: 40},
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryMVFR, OnTimeCount: 30, Flights: 40},
	})
	assertOutlookWeatherSide(t, "adverse better", better.Origin, floatPtr(0.25), floatPtr(0.75), floatPtr(0.5), 40, model.ConfidenceMedium)
}

func TestWeatherSideConfidenceBands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fair    int
		adverse int
		want    string
	}{
		{name: "thinner 29", fair: 29, adverse: 100, want: model.ConfidenceLow},
		{name: "thinner 30", fair: 100, adverse: 30, want: model.ConfidenceMedium},
		{name: "thinner 49", fair: 49, adverse: 80, want: model.ConfidenceMedium},
		{name: "thinner 50", fair: 50, adverse: 80, want: model.ConfidenceHigh},
		{name: "missing fair", fair: 0, adverse: 50, want: model.ConfidenceUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AssembleRouteOutlookWeather([]WeatherCategoryCount{
				{Side: WeatherSideOrigin, Category: model.WeatherCategoryVFRFair, OnTimeCount: tt.fair, Flights: tt.fair},
				{Side: WeatherSideOrigin, Category: model.WeatherCategoryIFR, OnTimeCount: tt.adverse, Flights: tt.adverse},
			})
			if got.Origin.Confidence != tt.want {
				t.Errorf("confidence = %q, want %q", got.Origin.Confidence, tt.want)
			}
		})
	}
}

func TestAssembleRouteOutlookWeatherCountsEveryAdverseCategory(t *testing.T) {
	t.Parallel()

	counts := make([]WeatherCategoryCount, 0, len(model.WeatherCategoryOrder())+1)
	adverse := 0

	for _, category := range model.WeatherCategoryOrder() {
		counts = append(counts, WeatherCategoryCount{
			Side:        WeatherSideOrigin,
			Category:    category,
			OnTimeCount: 1,
			Flights:     1,
		})

		if category != model.WeatherCategoryVFRFair && category != model.WeatherCategoryUnknown {
			adverse++
		}
	}

	counts = append(counts, WeatherCategoryCount{
		Side:        WeatherSideOrigin,
		Category:    WeatherUnmatched,
		OnTimeCount: 1,
		Flights:     4,
	})

	got := AssembleRouteOutlookWeather(counts)
	assertOutlookWeatherSide(t, "all categories", got.Origin, floatPtr(1), floatPtr(1), floatPtr(0), adverse, model.ConfidenceLow)

	if adverse == 0 {
		t.Fatal("expected adverse categories in the rollup order")
	}
}

func assertOutlookWeatherSide(t *testing.T, name string, got model.RouteOutlookWeatherSide, fair, adverse, delta *float64, flights int, confidence string) {
	t.Helper()

	assertOptionalRate(t, name+" fair", got.FairOnTimeRate, fair)
	assertOptionalRate(t, name+" adverse", got.AdverseOnTimeRate, adverse)
	assertOptionalRate(t, name+" delta", got.AdverseDelta, delta)

	if got.AdverseFlights != flights || got.Confidence != confidence {
		t.Errorf("%s flights = %d confidence %q, want flights %d confidence %q", name, got.AdverseFlights, got.Confidence, flights, confidence)
	}
}

func assertOptionalRate(t *testing.T, name string, got, want *float64) {
	t.Helper()

	if want == nil {
		if got != nil {
			t.Errorf("%s = %v, want null", name, *got)
		}

		return
	}

	if got == nil || *got != *want {
		t.Errorf("%s = %v, want %v", name, got, *want)
	}
}

func jsonNullField(t *testing.T, body []byte, field string) bool {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	value, ok := raw[field]
	if !ok {
		t.Fatalf("missing %s in %s", field, body)
	}

	var rate *float64
	if err := json.Unmarshal(value, &rate); err != nil {
		t.Fatalf("decode %s: %v", field, err)
	}

	return rate == nil
}
