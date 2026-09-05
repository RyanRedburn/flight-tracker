package postgres

import (
	"context"
	"database/sql"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const (
	carrierStatsExtraPlaceholder         = "/*extra*/"
	carrierStatsOriginAirportPlaceholder = "/*origin_airport*/"
	carrierStatsDestAirportPlaceholder   = "/*dest_airport*/"
)

func (s *Store) CarrierStats(ctx context.Context, filter store.CarrierStatsFilter) (*model.CarrierStats, error) {
	stats := emptyCarrierStats(filter)

	if filter.StartDate == "" && filter.EndDate == "" {
		var analysisEnd sql.NullString

		err := s.db.QueryRowContext(ctx, store.QueryCarrierStatsMaxDate, filter.Carrier).Scan(&analysisEnd)
		if err != nil {
			return nil, err
		}

		if !analysisEnd.Valid || analysisEnd.String == "" {
			return stats, nil
		}

		start, end, ok := store.CarrierStatsWindow(analysisEnd.String)
		if !ok {
			return stats, nil
		}

		filter.StartDate = start
		filter.EndDate = end
		stats.StartDate = start
		stats.EndDate = end
	}

	if err := s.scanCarrierOverall(ctx, filter, stats); err != nil {
		return nil, err
	}

	routes, err := s.listCarrierRoutes(ctx, filter)
	if err != nil {
		return nil, err
	}

	airports, err := s.listCarrierAirports(ctx, filter)
	if err != nil {
		return nil, err
	}

	stats.BestRoutes, stats.WorstRoutes = store.RankCarrierRoutes(routes)
	stats.BestAirports, stats.WorstAirports = store.RankCarrierAirports(airports)

	stats.RoundForResponse()

	return stats, nil
}

func emptyCarrierStats(filter store.CarrierStatsFilter) *model.CarrierStats {
	return &model.CarrierStats{
		Carrier:       filter.Carrier,
		StartDate:     filter.StartDate,
		EndDate:       filter.EndDate,
		Filters:       model.CarrierStatsFilters{State: filter.State},
		BestRoutes:    []model.CarrierRouteStat{},
		WorstRoutes:   []model.CarrierRouteStat{},
		BestAirports:  []model.CarrierAirportStat{},
		WorstAirports: []model.CarrierAirportStat{},
	}
}

func (s *Store) scanCarrierOverall(ctx context.Context, filter store.CarrierStatsFilter, stats *model.CarrierStats) error {
	query, args := buildCarrierStatsQuery(store.QueryCarrierStats, filter)

	var (
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
	)

	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.Flights,
		&stats.OnTime,
		&stats.Delayed,
		&stats.Cancelled,
		&stats.Diverted,
		&avgArr,
		&medianArr,
		&avgArrDelayed,
		&medianArrDelayed,
		&avgDep,
		&avgDepDelayed,
		&causeCarrier,
		&causeWeather,
		&causeNAS,
		&causeSecurity,
		&causeLate,
		&shareCarrier,
		&shareWeather,
		&shareNAS,
		&shareSecurity,
		&shareLate,
		&shareUnattributed,
	)
	if err != nil {
		return err
	}

	stats.OnTimeRate = ratio(stats.OnTime, stats.Flights)
	stats.DelayRate = ratio(stats.Delayed, stats.Flights)
	stats.CancellationRate = ratio(stats.Cancelled, stats.Flights)
	stats.DiversionRate = ratio(stats.Diverted, stats.Flights)
	stats.AvgArrivalDelayMinutes = nullFloat(avgArr)
	stats.MedianArrivalDelayMinutes = nullFloat(medianArr)
	stats.AvgArrivalDelayWhenDelayed = nullFloat(avgArrDelayed)
	stats.MedianArrivalDelayWhenDelayed = nullFloat(medianArrDelayed)
	stats.AvgDepartureDelayMinutes = nullFloat(avgDep)
	stats.AvgDepartureDelayWhenDelayed = nullFloat(avgDepDelayed)

	if stats.Delayed > 0 {
		stats.DelayCausesAvgMinutes = model.DelayCausesAvgMinutes{
			Carrier:      nullFloat(causeCarrier),
			Weather:      nullFloat(causeWeather),
			NAS:          nullFloat(causeNAS),
			Security:     nullFloat(causeSecurity),
			LateAircraft: nullFloat(causeLate),
		}
		stats.DelayCausesShare = store.DelayCausesShareFromCounts(
			stats.Delayed,
			shareCarrier,
			shareWeather,
			shareNAS,
			shareSecurity,
			shareLate,
			shareUnattributed,
		)
	}

	return nil
}

func (s *Store) listCarrierRoutes(ctx context.Context, filter store.CarrierStatsFilter) ([]model.CarrierRouteStat, error) {
	query, args := buildCarrierStatsQuery(store.QueryCarrierStatsRoutes, filter)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.CarrierRouteStat, 0)

	for rows.Next() {
		var item model.CarrierRouteStat
		if err := rows.Scan(
			&item.Origin,
			&item.Dest,
			&item.Flights,
			&item.OnTime,
			&item.Delayed,
			&item.Cancelled,
			&item.Diverted,
		); err != nil {
			return nil, err
		}

		item.OnTimeRate = ratio(item.OnTime, item.Flights)
		item.DelayRate = ratio(item.Delayed, item.Flights)
		item.CancellationRate = ratio(item.Cancelled, item.Flights)
		item.DiversionRate = ratio(item.Diverted, item.Flights)
		out = append(out, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func (s *Store) listCarrierAirports(ctx context.Context, filter store.CarrierStatsFilter) ([]model.CarrierAirportStat, error) {
	query, args := buildCarrierStatsQuery(store.QueryCarrierStatsAirports, filter)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.CarrierAirportStat, 0)

	for rows.Next() {
		var item model.CarrierAirportStat
		if err := rows.Scan(
			&item.Airport,
			&item.Flights,
			&item.OnTime,
			&item.Delayed,
			&item.Cancelled,
			&item.Diverted,
		); err != nil {
			return nil, err
		}

		item.OnTimeRate = ratio(item.OnTime, item.Flights)
		item.DelayRate = ratio(item.Delayed, item.Flights)
		item.CancellationRate = ratio(item.Cancelled, item.Flights)
		item.DiversionRate = ratio(item.Diverted, item.Flights)
		out = append(out, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func buildCarrierStatsQuery(template string, filter store.CarrierStatsFilter) (string, []any) {
	args := []any{filter.Carrier, filter.StartDate, filter.EndDate}

	extra := ""
	originAirport := "TRUE"
	destAirport := "TRUE"

	if filter.State != "" {
		extra = " AND (origin_state = $4 OR dest_state = $4)"
		originAirport = "origin_state = $4"
		destAirport = "dest_state = $4"
		args = append(args, filter.State)
	}

	query := strings.Replace(template, carrierStatsExtraPlaceholder, extra, 1)
	query = strings.Replace(query, carrierStatsOriginAirportPlaceholder, originAirport, 1)
	query = strings.Replace(query, carrierStatsDestAirportPlaceholder, destAirport, 1)

	return query, args
}
