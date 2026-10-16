package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
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
	}

	stats.Analysis.Start = filter.StartDate
	stats.Analysis.End = filter.EndDate

	if err := s.applyCarrierAggregates(ctx, filter, stats); err != nil {
		return nil, err
	}

	stats.RoundForResponse()

	return stats, nil
}

func emptyCarrierStats(filter store.CarrierStatsFilter) *model.CarrierStats {
	return &model.CarrierStats{
		Filters: model.Filters{
			Carrier:   filter.Carrier,
			State:     filter.State,
			StartDate: filter.StartDate,
			EndDate:   filter.EndDate,
		},
		BestRoutes:    []model.CarrierRouteStat{},
		WorstRoutes:   []model.CarrierRouteStat{},
		BestAirports:  []model.CarrierAirportStat{},
		WorstAirports: []model.CarrierAirportStat{},
	}
}

func (s *Store) applyCarrierAggregates(ctx context.Context, filter store.CarrierStatsFilter, stats *model.CarrierStats) error {
	query, args := buildCarrierStatsQuery(store.QueryCarrierStats, filter)

	var (
		scanned      delayStatsScan
		routesJSON   []byte
		airportsJSON []byte
	)

	if err := s.db.QueryRowContext(ctx, query, args...).Scan(append(scanned.args(), &routesJSON, &airportsJSON)...); err != nil {
		return err
	}

	applyDelayStats(scanned, carrierDelayFields(stats))
	stats.Analysis.SampleSize = stats.Flights

	routes, err := unmarshalCarrierRoutes(routesJSON)
	if err != nil {
		return fmt.Errorf("decode carrier routes: %w", err)
	}

	airports, err := unmarshalCarrierAirports(airportsJSON)
	if err != nil {
		return fmt.Errorf("decode carrier airports: %w", err)
	}

	stats.BestRoutes, stats.WorstRoutes = store.RankCarrierRoutes(routes)
	stats.BestAirports, stats.WorstAirports = store.RankCarrierAirports(airports)

	return nil
}

func unmarshalCarrierRoutes(raw []byte) ([]model.CarrierRouteStat, error) {
	rows, err := decodeCarrierCountRows[model.CarrierRouteStat](raw)
	if err != nil {
		return nil, err
	}

	for i := range rows {
		rows[i].OnTimeRate = ratio(rows[i].OnTime, rows[i].Flights)
		rows[i].DelayRate = ratio(rows[i].Delayed, rows[i].Flights)
		rows[i].CancellationRate = ratio(rows[i].Cancelled, rows[i].Flights)
		rows[i].DiversionRate = ratio(rows[i].Diverted, rows[i].Flights)
	}

	return rows, nil
}

func unmarshalCarrierAirports(raw []byte) ([]model.CarrierAirportStat, error) {
	rows, err := decodeCarrierCountRows[model.CarrierAirportStat](raw)
	if err != nil {
		return nil, err
	}

	for i := range rows {
		rows[i].OnTimeRate = ratio(rows[i].OnTime, rows[i].Flights)
		rows[i].DelayRate = ratio(rows[i].Delayed, rows[i].Flights)
		rows[i].CancellationRate = ratio(rows[i].Cancelled, rows[i].Flights)
		rows[i].DiversionRate = ratio(rows[i].Diverted, rows[i].Flights)
	}

	return rows, nil
}

func decodeCarrierCountRows[T any](raw []byte) ([]T, error) {
	if len(raw) == 0 || string(raw) == jsonNull {
		return []T{}, nil
	}

	var rows []T
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}

	if rows == nil {
		return []T{}, nil
	}

	return rows, nil
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
