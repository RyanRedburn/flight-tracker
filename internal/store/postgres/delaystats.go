package postgres

import (
	"database/sql"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type delayStatsScan struct {
	flights           int
	onTime            int
	delayed           int
	cancelled         int
	diverted          int
	avgArr            sql.NullFloat64
	medianArr         sql.NullFloat64
	avgArrDelayed     sql.NullFloat64
	medianArrDelayed  sql.NullFloat64
	avgDep            sql.NullFloat64
	avgDepDelayed     sql.NullFloat64
	causeCarrier      sql.NullFloat64
	causeWeather      sql.NullFloat64
	causeNAS          sql.NullFloat64
	causeSecurity     sql.NullFloat64
	causeLate         sql.NullFloat64
	shareCarrier      int
	shareWeather      int
	shareNAS          int
	shareSecurity     int
	shareLate         int
	shareUnattributed int
}

func (row *delayStatsScan) args() []any {
	return []any{
		&row.flights,
		&row.onTime,
		&row.delayed,
		&row.cancelled,
		&row.diverted,
		&row.avgArr,
		&row.medianArr,
		&row.avgArrDelayed,
		&row.medianArrDelayed,
		&row.avgDep,
		&row.avgDepDelayed,
		&row.causeCarrier,
		&row.causeWeather,
		&row.causeNAS,
		&row.causeSecurity,
		&row.causeLate,
		&row.shareCarrier,
		&row.shareWeather,
		&row.shareNAS,
		&row.shareSecurity,
		&row.shareLate,
		&row.shareUnattributed,
	}
}

type delayStatFields struct {
	flights                       *int
	onTime                        *int
	delayed                       *int
	cancelled                     *int
	diverted                      *int
	onTimeRate                    *float64
	delayRate                     *float64
	cancellationRate              *float64
	diversionRate                 *float64
	avgArrivalDelayMinutes        *float64
	medianArrivalDelayMinutes     *float64
	avgArrivalDelayWhenDelayed    *float64
	medianArrivalDelayWhenDelayed *float64
	avgDepartureDelayMinutes      *float64
	avgDepartureDelayWhenDelayed  *float64
	causes                        *model.DelayCausesAvgMinutes
	share                         *model.DelayCausesShare
}

func applyDelayStats(row delayStatsScan, dst delayStatFields) {
	*dst.flights = row.flights
	*dst.onTime = row.onTime
	*dst.delayed = row.delayed
	*dst.cancelled = row.cancelled
	*dst.diverted = row.diverted
	*dst.onTimeRate = ratio(row.onTime, row.flights)
	*dst.delayRate = ratio(row.delayed, row.flights)
	*dst.cancellationRate = ratio(row.cancelled, row.flights)
	*dst.diversionRate = ratio(row.diverted, row.flights)
	*dst.avgArrivalDelayMinutes = nullFloat(row.avgArr)
	*dst.medianArrivalDelayMinutes = nullFloat(row.medianArr)
	*dst.avgArrivalDelayWhenDelayed = nullFloat(row.avgArrDelayed)
	*dst.medianArrivalDelayWhenDelayed = nullFloat(row.medianArrDelayed)
	*dst.avgDepartureDelayMinutes = nullFloat(row.avgDep)
	*dst.avgDepartureDelayWhenDelayed = nullFloat(row.avgDepDelayed)

	if row.delayed == 0 {
		return
	}

	*dst.causes = model.DelayCausesAvgMinutes{
		Carrier:      nullFloat(row.causeCarrier),
		Weather:      nullFloat(row.causeWeather),
		NAS:          nullFloat(row.causeNAS),
		Security:     nullFloat(row.causeSecurity),
		LateAircraft: nullFloat(row.causeLate),
	}
	*dst.share = store.DelayCausesShareFromCounts(
		row.delayed,
		row.shareCarrier,
		row.shareWeather,
		row.shareNAS,
		row.shareSecurity,
		row.shareLate,
		row.shareUnattributed,
	)
}

func routeDelayFields(stats *model.RouteStats) delayStatFields {
	return delayStatFields{
		flights:                       &stats.Flights,
		onTime:                        &stats.OnTime,
		delayed:                       &stats.Delayed,
		cancelled:                     &stats.Cancelled,
		diverted:                      &stats.Diverted,
		onTimeRate:                    &stats.OnTimeRate,
		delayRate:                     &stats.DelayRate,
		cancellationRate:              &stats.CancellationRate,
		diversionRate:                 &stats.DiversionRate,
		avgArrivalDelayMinutes:        &stats.AvgArrivalDelayMinutes,
		medianArrivalDelayMinutes:     &stats.MedianArrivalDelayMinutes,
		avgArrivalDelayWhenDelayed:    &stats.AvgArrivalDelayWhenDelayed,
		medianArrivalDelayWhenDelayed: &stats.MedianArrivalDelayWhenDelayed,
		avgDepartureDelayMinutes:      &stats.AvgDepartureDelayMinutes,
		avgDepartureDelayWhenDelayed:  &stats.AvgDepartureDelayWhenDelayed,
		causes:                        &stats.DelayCausesAvgMinutes,
		share:                         &stats.DelayCausesShare,
	}
}

func carrierDelayFields(stats *model.CarrierStats) delayStatFields {
	return delayStatFields{
		flights:                       &stats.Flights,
		onTime:                        &stats.OnTime,
		delayed:                       &stats.Delayed,
		cancelled:                     &stats.Cancelled,
		diverted:                      &stats.Diverted,
		onTimeRate:                    &stats.OnTimeRate,
		delayRate:                     &stats.DelayRate,
		cancellationRate:              &stats.CancellationRate,
		diversionRate:                 &stats.DiversionRate,
		avgArrivalDelayMinutes:        &stats.AvgArrivalDelayMinutes,
		medianArrivalDelayMinutes:     &stats.MedianArrivalDelayMinutes,
		avgArrivalDelayWhenDelayed:    &stats.AvgArrivalDelayWhenDelayed,
		medianArrivalDelayWhenDelayed: &stats.MedianArrivalDelayWhenDelayed,
		avgDepartureDelayMinutes:      &stats.AvgDepartureDelayMinutes,
		avgDepartureDelayWhenDelayed:  &stats.AvgDepartureDelayWhenDelayed,
		causes:                        &stats.DelayCausesAvgMinutes,
		share:                         &stats.DelayCausesShare,
	}
}
