package store

import (
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

func TestAssembleRouteWeatherStatsBothSides(t *testing.T) {
	t.Parallel()

	got := AssembleRouteWeatherStats(RouteStatsFilter{
		Origin:       testTravelOrigin,
		Dest:         testTravelDest,
		StartDate:    "2026-01-01",
		EndDate:      "2026-01-31",
		Carrier:      "UA",
		FlightNumber: "100",
		DaysOfWeek:   []int{1, 5},
	}, []WeatherCategoryCount{
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryThunder, OnTimeCount: 1, Flights: 4},
		{Side: WeatherSideOrigin, Category: model.WeatherCategoryVFRFair, OnTimeCount: 2, Flights: 3},
		{Side: WeatherSideOrigin, Category: WeatherUnmatched, OnTimeCount: 0, Flights: 2},
		{Side: WeatherSideDest, Category: model.WeatherCategoryIFR, OnTimeCount: 1, Flights: 2},
		{Side: WeatherSideDest, Category: model.WeatherCategoryMVFR, OnTimeCount: 3, Flights: 6},
		{Side: WeatherSideDest, Category: WeatherUnmatched, OnTimeCount: 1, Flights: 1},
	})

	if got.Origin != testTravelOrigin || got.Dest != testTravelDest || got.Filters.Carrier != "UA" || got.Filters.FlightNumber != "100" {
		t.Fatalf("identity = %+v", got)
	}

	if len(got.Filters.DaysOfWeek) != 2 || got.Filters.DaysOfWeek[0] != 1 {
		t.Fatalf("days = %#v", got.Filters.DaysOfWeek)
	}

	if got.Flights != 9 {
		t.Fatalf("flights = %d, want 9", got.Flights)
	}

	if got.OriginWeather.Flights != 9 || got.OriginWeather.FlightsUnmatched != 2 {
		t.Fatalf("origin side = %+v", got.OriginWeather)
	}

	if got.DestWeather.Flights != 9 || got.DestWeather.FlightsUnmatched != 1 {
		t.Fatalf("dest side = %+v", got.DestWeather)
	}

	thunder := categoryStat(t, got.OriginWeather, model.WeatherCategoryThunder)
	if thunder.Flights != 4 || thunder.OnTimeRate != 0.25 {
		t.Fatalf("origin thunder = %+v", thunder)
	}

	fair := categoryStat(t, got.OriginWeather, model.WeatherCategoryVFRFair)
	if fair.Flights != 3 || fair.OnTimeRate != 0.67 {
		t.Fatalf("origin vfr = %+v", fair)
	}

	ifr := categoryStat(t, got.DestWeather, model.WeatherCategoryIFR)
	if ifr.Flights != 2 || ifr.OnTimeRate != 0.5 {
		t.Fatalf("dest ifr = %+v", ifr)
	}

	if categoryStat(t, got.OriginWeather, model.WeatherCategoryIFR).Flights != 0 {
		t.Fatal("missing origin category should be a zero bucket")
	}

	if len(got.OriginWeather.Categories) != len(model.WeatherCategoryOrder()) {
		t.Fatalf("origin categories = %d", len(got.OriginWeather.Categories))
	}

	for _, stat := range got.OriginWeather.Categories {
		if stat.Category == WeatherUnmatched {
			t.Fatal("unmatched must not appear as a category")
		}
	}
}

func TestAssembleRouteWeatherStatsEmpty(t *testing.T) {
	t.Parallel()

	got := AssembleRouteWeatherStats(RouteStatsFilter{
		Origin:    testTravelOrigin,
		Dest:      testTravelDest,
		StartDate: "2026-01-01",
		EndDate:   "2026-01-02",
	}, nil)

	if got.Flights != 0 || got.OriginWeather.FlightsUnmatched != 0 || got.DestWeather.Flights != 0 {
		t.Fatalf("empty = %+v", got)
	}

	if got.Filters.DaysOfWeek == nil || len(got.OriginWeather.Categories) != len(model.WeatherCategoryOrder()) {
		t.Fatalf("empty shape = %+v", got)
	}

	for _, stat := range got.DestWeather.Categories {
		if stat.Flights != 0 || stat.OnTimeRate != 0 {
			t.Fatalf("zero bucket = %+v", stat)
		}
	}
}

func TestAssembleRouteWeatherStatsIgnoresUnknownSide(t *testing.T) {
	t.Parallel()

	got := AssembleRouteWeatherStats(RouteStatsFilter{
		Origin: testTravelOrigin,
		Dest:   testTravelDest,
	}, []WeatherCategoryCount{
		{Side: "enroute", Category: model.WeatherCategoryVFRFair, OnTimeCount: 1, Flights: 1},
		{Side: WeatherSideOrigin, Category: "FOG", OnTimeCount: 1, Flights: 5},
	})

	if got.Flights != 0 || got.OriginWeather.FlightsUnmatched != 0 {
		t.Fatalf("unknown buckets counted: %+v", got.OriginWeather)
	}
}

func categoryStat(t *testing.T, side model.WeatherSideStats, category string) model.WeatherCategoryStat {
	t.Helper()

	for _, stat := range side.Categories {
		if stat.Category == category {
			return stat
		}
	}

	t.Fatalf("category %s missing", category)

	return model.WeatherCategoryStat{}
}
