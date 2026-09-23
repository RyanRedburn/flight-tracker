package model

import (
	"encoding/json"
	"testing"
)

const jsonCarrierOnTime = "carrier_on_time"

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
		DelayCausesShare: DelayCausesShare{
			Carrier:      1.0 / 3.0,
			Weather:      0.125,
			NAS:          2.0 / 3.0,
			Security:     0,
			LateAircraft: 0.5,
			Unattributed: 0.04,
		},
		CarrierOnTime: []CarrierOnTime{
			{OnTimeRate: 1.0 / 3.0},
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
	assertFloat(t, "delay_causes_share.carrier", stats.DelayCausesShare.Carrier, 0.33)
	assertFloat(t, "delay_causes_share.weather", stats.DelayCausesShare.Weather, 0.13)
	assertFloat(t, "delay_causes_share.nas", stats.DelayCausesShare.NAS, 0.67)
	assertFloat(t, "delay_causes_share.security", stats.DelayCausesShare.Security, 0)
	assertFloat(t, "delay_causes_share.late_aircraft", stats.DelayCausesShare.LateAircraft, 0.5)
	assertFloat(t, "delay_causes_share.unattributed", stats.DelayCausesShare.Unattributed, 0.04)
	assertFloat(t, "carrier_on_time[0].on_time_rate", stats.CarrierOnTime[0].OnTimeRate, 0.33)
}

func TestRouteStatsCarrierOnTimeJSON(t *testing.T) {
	omitted, err := json.Marshal(RouteStats{})
	if err != nil {
		t.Fatalf("marshal omitted: %v", err)
	}

	if jsonHasField(t, omitted, jsonCarrierOnTime) {
		t.Fatalf("carrier_on_time should be omitted when nil: %s", omitted)
	}

	empty, err := json.Marshal(RouteStats{
		CarrierOnTime: []CarrierOnTime{},
	})
	if err != nil {
		t.Fatalf("marshal empty: %v", err)
	}

	raw := jsonField(t, empty, jsonCarrierOnTime)
	if string(raw) != "[]" {
		t.Fatalf("empty carrier_on_time = %s, want []", raw)
	}

	populated, err := json.Marshal(RouteStats{
		CarrierOnTime: []CarrierOnTime{{
			Carrier:    "UA",
			OnTimeRate: 0.82,
			Flights:    1204,
		}},
	})
	if err != nil {
		t.Fatalf("marshal populated: %v", err)
	}

	var rows []CarrierOnTime
	if err := json.Unmarshal(jsonField(t, populated, jsonCarrierOnTime), &rows); err != nil {
		t.Fatalf("decode carrier_on_time: %v", err)
	}

	if len(rows) != 1 || rows[0].Carrier != "UA" || rows[0].OnTimeRate != 0.82 || rows[0].Flights != 1204 {
		t.Fatalf("carrier_on_time = %+v", rows)
	}
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

func TestRouteTravelWindowsRoundAndJSON(t *testing.T) {
	windows := RouteTravelWindows{
		Origin: "SEA",
		Dest:   "MIA",
		ByHour: []TravelWindowHourBucket{
			{Hour: 0, OnTimeRate: 1.0 / 3.0, Flights: 40},
		},
		BestHours: []TravelWindowHourBucket{
			{Hour: 0, OnTimeRate: 1.0 / 3.0, Flights: 40},
		},
	}

	windows.RoundForResponse()
	assertFloat(t, "by_hour[0].on_time_rate", windows.ByHour[0].OnTimeRate, 0.33)
	assertFloat(t, "best_hours[0].on_time_rate", windows.BestHours[0].OnTimeRate, 0.33)

	omitted, err := json.Marshal(windows)
	if err != nil {
		t.Fatalf("marshal omitted carrier: %v", err)
	}

	if jsonHasField(t, omitted, "carrier") {
		t.Fatalf("carrier should be omitted when empty: %s", omitted)
	}

	hourRaw := jsonField(t, omitted, "by_hour")

	var hours []TravelWindowHourBucket
	if err := json.Unmarshal(hourRaw, &hours); err != nil {
		t.Fatalf("decode by_hour: %v", err)
	}

	if len(hours) != 1 || hours[0].Hour != 0 {
		t.Fatalf("by_hour = %+v, want hour 0 present", hours)
	}

	withCarrier, err := json.Marshal(RouteTravelWindows{Origin: "SEA", Dest: "MIA", Carrier: "AS"})
	if err != nil {
		t.Fatalf("marshal carrier: %v", err)
	}

	if !jsonHasField(t, withCarrier, "carrier") {
		t.Fatalf("carrier missing: %s", withCarrier)
	}
}

func assertFloat(t *testing.T, name string, got, want float64) {
	t.Helper()

	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func jsonHasField(t *testing.T, body []byte, field string) bool {
	t.Helper()

	_, ok := jsonObject(t, body)[field]

	return ok
}

func jsonField(t *testing.T, body []byte, field string) json.RawMessage {
	t.Helper()

	raw, ok := jsonObject(t, body)[field]
	if !ok {
		t.Fatalf("missing field %q in %s", field, body)
	}

	return raw
}

func jsonObject(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}

	return raw
}
