package model

import "testing"

func TestRouteStatsRoundForResponse(t *testing.T) {
	stats := RouteStats{
		OnTimeRate:                    1.0 / 3.0,
		DelayRate:                     2.0 / 3.0,
		CancellationRate:              0.125,
		DiversionRate:                 0.5,
		AvgArrivalDelayMinutes:        15.4,
		MedianArrivalDelayMinutes:     15.5,
		AvgArrivalDelayWhenDelayed:    -1.5,
		MedianArrivalDelayWhenDelayed: 0.4,
		AvgDepartureDelayMinutes:      10.49,
		AvgDepartureDelayWhenDelayed:  10.5,
		DelayCausesAvgMinutes: DelayCausesAvgMinutes{
			Carrier:      1.4,
			Weather:      2.5,
			NAS:          3.6,
			Security:     0.4,
			LateAircraft: 9.5,
		},
	}

	stats.RoundForResponse()

	assertFloat(t, "on_time_rate", stats.OnTimeRate, 0.33)
	assertFloat(t, "delay_rate", stats.DelayRate, 0.67)
	assertFloat(t, "cancellation_rate", stats.CancellationRate, 0.13)
	assertFloat(t, "diversion_rate", stats.DiversionRate, 0.5)
	assertFloat(t, "avg_arrival_delay_minutes", stats.AvgArrivalDelayMinutes, 15)
	assertFloat(t, "median_arrival_delay_minutes", stats.MedianArrivalDelayMinutes, 16)
	assertFloat(t, "avg_arrival_delay_when_delayed", stats.AvgArrivalDelayWhenDelayed, -2)
	assertFloat(t, "median_arrival_delay_when_delayed", stats.MedianArrivalDelayWhenDelayed, 0)
	assertFloat(t, "avg_departure_delay_minutes", stats.AvgDepartureDelayMinutes, 10)
	assertFloat(t, "avg_departure_delay_when_delayed", stats.AvgDepartureDelayWhenDelayed, 11)
	assertFloat(t, "delay_causes.carrier", stats.DelayCausesAvgMinutes.Carrier, 1)
	assertFloat(t, "delay_causes.weather", stats.DelayCausesAvgMinutes.Weather, 3)
	assertFloat(t, "delay_causes.nas", stats.DelayCausesAvgMinutes.NAS, 4)
	assertFloat(t, "delay_causes.security", stats.DelayCausesAvgMinutes.Security, 0)
	assertFloat(t, "delay_causes.late_aircraft", stats.DelayCausesAvgMinutes.LateAircraft, 10)
}

func TestRouteOutlookRoundForResponse(t *testing.T) {
	out := RouteOutlook{
		OnTimeProbability:             1.0 / 3.0,
		DelayProbability:              2.0 / 3.0,
		CancellationProbability:       0.125,
		DiversionProbability:          0.5,
		LikelyArrivalDelayMinutes:     15.4,
		MedianArrivalDelayMinutes:     15.5,
		LikelyArrivalDelayWhenDelayed: -1.5,
		MedianArrivalDelayWhenDelayed: 0.4,
		LikelyDepartureDelayMinutes:   10.5,
		DepTimeWindowMinutes:          30,
	}

	out.RoundForResponse()

	assertFloat(t, "on_time_probability", out.OnTimeProbability, 0.33)
	assertFloat(t, "delay_probability", out.DelayProbability, 0.67)
	assertFloat(t, "cancellation_probability", out.CancellationProbability, 0.13)
	assertFloat(t, "diversion_probability", out.DiversionProbability, 0.5)
	assertFloat(t, "likely_arrival_delay_minutes", out.LikelyArrivalDelayMinutes, 15)
	assertFloat(t, "median_arrival_delay_minutes", out.MedianArrivalDelayMinutes, 16)
	assertFloat(t, "likely_arrival_delay_when_delayed", out.LikelyArrivalDelayWhenDelayed, -2)
	assertFloat(t, "median_arrival_delay_when_delayed", out.MedianArrivalDelayWhenDelayed, 0)
	assertFloat(t, "likely_departure_delay_minutes", out.LikelyDepartureDelayMinutes, 11)

	if out.DepTimeWindowMinutes != 30 {
		t.Errorf("dep_time_window_minutes = %d, want 30 (input filter, not rounded as a delay)", out.DepTimeWindowMinutes)
	}
}

func assertFloat(t *testing.T, name string, got, want float64) {
	t.Helper()

	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}
