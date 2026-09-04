package iem

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

var (
	ErrNoFlightAirports        = errors.New("no flight airport codes found; ingest flight performance data first or provide stations")
	ErrNoMatchedStations       = errors.New("no flight airports matched IEM ASOS station ids")
	ErrNoWeatherStationMapping = errors.New("no weather station mapping found; ingest weather-stations first or provide stations")
)

// StationResolver builds a default IEM station list from persisted BTS↔ASOS mapping rows.
type StationResolver struct {
	store  store.Store
	logger *slog.Logger
}

func NewStationResolver(s store.Store, logger *slog.Logger) *StationResolver {
	if logger == nil {
		logger = slog.Default()
	}

	return &StationResolver{
		store:  s,
		logger: logger,
	}
}

// Resolve returns matched IEM station ids and unmatched flight airport codes.
func (r *StationResolver) Resolve(ctx context.Context) (stations []string, unmatched []string, err error) {
	if r == nil || r.store == nil {
		return nil, nil, errors.New("station resolver not configured")
	}

	rows, err := r.store.ListAirportWeatherStations(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list airport weather stations: %w", err)
	}

	if len(rows) == 0 {
		return nil, nil, ErrNoWeatherStationMapping
	}

	stations = make([]string, 0, len(rows))
	unmatched = make([]string, 0)

	for _, row := range rows {
		if !row.Matched {
			unmatched = append(unmatched, row.AirportCode)
			continue
		}

		sid := strings.ToUpper(strings.TrimSpace(row.IEMSID))
		if sid == "" {
			sid = strings.ToUpper(strings.TrimSpace(row.AirportCode))
		}

		stations = append(stations, sid)
	}

	sort.Strings(stations)
	sort.Strings(unmatched)

	if len(stations) == 0 {
		return nil, unmatched, ErrNoMatchedStations
	}

	if len(unmatched) > 0 {
		r.logger.Warn("weather station resolve: flight airports without IEM ASOS sid",
			"unmatched_count", len(unmatched),
			"matched_count", len(stations),
			"unmatched", unmatched,
		)
	}

	return stations, unmatched, nil
}

func mapFlightAirports(flightCodes []string, catalog map[string]Station, refs map[string]store.AirportIdentifiers) []store.AirportWeatherStation {
	rows := make([]store.AirportWeatherStation, 0, len(flightCodes))
	seen := make(map[string]struct{}, len(flightCodes))

	for _, code := range flightCodes {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}

		if _, ok := seen[code]; ok {
			continue
		}

		seen[code] = struct{}{}

		row := store.AirportWeatherStation{AirportCode: code}
		if station, ok := lookupStation(catalog, stationCandidates(code, refs)); ok {
			row.Matched = true
			row.IEMSID = station.SID
			row.TzName = station.TzName
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].AirportCode < rows[j].AirportCode
	})

	return rows
}

func stationCandidates(code string, refs map[string]store.AirportIdentifiers) []string {
	out := make([]string, 0, 4)
	seen := make(map[string]struct{}, 4)

	add := func(v string) {
		v = strings.ToUpper(strings.TrimSpace(v))
		if v == "" {
			return
		}

		if _, ok := seen[v]; ok {
			return
		}

		seen[v] = struct{}{}
		out = append(out, v)
	}

	add(code)

	if refs != nil {
		if ref, ok := refs[code]; ok {
			add(ref.LocalCode)
			add(ref.ICAOCode)
			add(ref.Ident)
		}
	}

	return out
}

func lookupStation(catalog map[string]Station, candidates []string) (Station, bool) {
	for _, candidate := range candidates {
		if station, ok := catalog[candidate]; ok {
			return station, true
		}
	}

	return Station{}, false
}
