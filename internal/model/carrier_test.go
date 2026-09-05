package model

import "testing"

func TestCarrierStatsRoundForResponse(t *testing.T) {
	stats := CarrierStats{
		OnTimeRate:             1.0 / 3.0,
		AvgArrivalDelayMinutes: 15.4,
		DelayCausesAvgMinutes:  DelayCausesAvgMinutes{Carrier: 1.4},
		DelayCausesShare:       DelayCausesShare{LateAircraft: 2.0 / 3.0, Unattributed: 1.0 / 3.0},
		BestRoutes:             []CarrierRouteStat{{OnTimeRate: 1.0 / 3.0}},
		WorstAirports:          []CarrierAirportStat{{DelayRate: 2.0 / 3.0}},
		WorstRoutes:            []CarrierRouteStat{},
		BestAirports:           []CarrierAirportStat{},
	}

	stats.RoundForResponse()

	assertFloat(t, "on_time_rate", stats.OnTimeRate, 0.33)
	assertFloat(t, "avg_arrival_delay_minutes", stats.AvgArrivalDelayMinutes, 15)
	assertFloat(t, "delay_causes.carrier", stats.DelayCausesAvgMinutes.Carrier, 1)
	assertFloat(t, "delay_causes_share.late_aircraft", stats.DelayCausesShare.LateAircraft, 0.67)
	assertFloat(t, "delay_causes_share.unattributed", stats.DelayCausesShare.Unattributed, 0.33)
	assertFloat(t, "best_routes[0].on_time_rate", stats.BestRoutes[0].OnTimeRate, 0.33)
	assertFloat(t, "worst_airports[0].delay_rate", stats.WorstAirports[0].DelayRate, 0.67)
}
