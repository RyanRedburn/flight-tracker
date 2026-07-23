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
	ErrNoFlightAirports  = errors.New("no flight airport codes found; ingest flight performance data first or provide stations")
	ErrNoMatchedStations = errors.New("no flight airports matched IEM ASOS station ids")
)

// StationResolver builds a default IEM station list from BTS airports ∩ US ASOS metadata.
type StationResolver struct {
	store   store.Store
	catalog *NetworkCatalog
	logger  *slog.Logger
}

func NewStationResolver(s store.Store, catalog *NetworkCatalog, logger *slog.Logger) *StationResolver {
	if logger == nil {
		logger = slog.Default()
	}

	return &StationResolver{
		store:   s,
		catalog: catalog,
		logger:  logger,
	}
}

// Resolve returns matched IEM station ids and unmatched flight airport codes.
func (r *StationResolver) Resolve(ctx context.Context) (stations []string, unmatched []string, err error) {
	if r == nil || r.store == nil || r.catalog == nil {
		return nil, nil, errors.New("station resolver not configured")
	}

	flightCodes, err := r.store.DistinctFlightAirportCodes(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list flight airports: %w", err)
	}

	if len(flightCodes) == 0 {
		return nil, nil, ErrNoFlightAirports
	}

	iemIDs, err := r.catalog.LoadStationIDs(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("load iem station catalog: %w", err)
	}

	stations, unmatched = intersectStations(flightCodes, iemIDs)
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

func intersectStations(flightCodes []string, iemIDs map[string]struct{}) (matched, unmatched []string) {
	matched = make([]string, 0, len(flightCodes))
	unmatched = make([]string, 0)

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

		if _, ok := iemIDs[code]; ok {
			matched = append(matched, code)
			continue
		}

		unmatched = append(unmatched, code)
	}

	sort.Strings(matched)
	sort.Strings(unmatched)

	return matched, unmatched
}
