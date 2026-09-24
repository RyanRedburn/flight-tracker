package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const (
	carrierStatsExtraPlaceholder         = "/*extra*/"
	carrierStatsOriginAirportPlaceholder = "/*origin_airport*/"
	carrierStatsDestAirportPlaceholder   = "/*dest_airport*/"
	carrierStatsMinSamplePlaceholder     = "/*min_sample*/"
)

func (s *Store) CarrierStats(ctx context.Context, filter store.CarrierStatsFilter) (*model.CarrierStats, error) {
	stats := emptyCarrierStats(filter)

	// MAX(flight_date) is null when the carrier has no rows. Date and state
	// filters are applied later and may still yield an empty success.
	var analysisEnd sql.NullString

	err := s.db.QueryRowContext(ctx, store.QueryCarrierStatsMaxDate, filter.Carrier).Scan(&analysisEnd)
	if err != nil {
		return nil, err
	}

	if !analysisEnd.Valid || analysisEnd.String == "" {
		return nil, fmt.Errorf("carrier stats %s: %w", filter.Carrier, store.ErrNotFound)
	}

	if filter.StartDate == "" && filter.EndDate == "" {
		start, end, ok := store.CarrierStatsWindow(analysisEnd.String)
		if !ok {
			return nil, fmt.Errorf("parse carrier stats max flight date %q", analysisEnd.String)
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

	var scanned delayStatsScan

	if err := s.db.QueryRowContext(ctx, query, args...).Scan(scanned.args()...); err != nil {
		return err
	}

	applyDelayStats(scanned, carrierDelayFields(stats))

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
	query = strings.ReplaceAll(query, carrierStatsMinSamplePlaceholder, strconv.Itoa(store.CarrierStatsMinSampleSize))

	return query, args
}
