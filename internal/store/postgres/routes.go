package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const (
	routeStatsExtraPlaceholder         = "/*extra*/"
	routeStatsCarrierOnTimePlaceholder = "/*carrier_on_time*/"
	jsonNull                           = "null"
)

func (s *Store) RouteStats(ctx context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error) {
	// COUNT(*) without GROUP BY is one row of zeros when nothing matches, so a
	// missing route is not sql.ErrNoRows. Identity ignores date and other filters.
	if err := s.ensureRouteIdentity(ctx, filter.Origin, filter.Dest, filter.Carrier); err != nil {
		return nil, err
	}

	stats := emptyRouteStats(filter)

	query, args := buildRouteStatsQuery(store.QueryRouteStats, filter)

	var (
		scanned       delayStatsScan
		diversionJSON []byte
		carrierJSON   []byte
	)

	err := s.db.QueryRowContext(ctx, query, args...).Scan(append(scanned.args(), &diversionJSON, &carrierJSON)...)
	if err != nil {
		return nil, err
	}

	applyDelayStats(scanned, routeDelayFields(stats))
	stats.Analysis.SampleSize = stats.Flights

	airports, err := unmarshalAirportCounts(diversionJSON)
	if err != nil {
		return nil, fmt.Errorf("decode diversion airports: %w", err)
	}

	stats.DiversionAirports = airports

	if filter.Carrier == "" {
		rows, err := unmarshalCarrierOnTime(carrierJSON)
		if err != nil {
			return nil, fmt.Errorf("decode carrier on time: %w", err)
		}

		stats.CarrierOnTime = rows
		stats.CarrierComparison = store.CarrierComparison(rows)
	}

	stats.RoundForResponse()

	return stats, nil
}

// RouteLatestFlightDate is the newest flight_date on the route. No row means the route is unknown.
func (s *Store) RouteLatestFlightDate(ctx context.Context, origin, dest string) (string, error) {
	var maxDate sql.NullString

	err := s.db.QueryRowContext(ctx, store.QueryRouteMaxFlightDate, origin, dest).Scan(&maxDate)
	if err != nil {
		return "", err
	}

	if !maxDate.Valid || maxDate.String == "" {
		return "", fmt.Errorf("route latest flight %s-%s: %w", origin, dest, store.ErrNotFound)
	}

	return maxDate.String, nil
}

func (s *Store) RouteOutlook(ctx context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
	out := emptyRouteOutlook(filter)

	var analysisEnd sql.NullString

	err := s.db.QueryRowContext(ctx, store.QueryRouteOutlookMaxDate,
		filter.Origin,
		filter.Dest,
		filter.Carrier,
	).Scan(&analysisEnd)
	if err != nil {
		return nil, err
	}

	if !analysisEnd.Valid || analysisEnd.String == "" {
		return nil, fmt.Errorf("route outlook %s-%s %s: %w", filter.Origin, filter.Dest, filter.Carrier, store.ErrNotFound)
	}

	var p90Arr sql.NullFloat64

	err = store.ApplyOutlookLookback(out, filter.DepTimeWindowSet, func(lookbackDays int) error {
		return s.fillOutlookSlot(ctx, out, filter, analysisEnd.String, lookbackDays, &p90Arr)
	})
	if err != nil {
		return nil, err
	}

	if store.OutlookSampleHasFlights(out.Analysis.SampleSize, out.Analysis.SampleReason) {
		if err := s.setRecommendedConnection(ctx, out, p90Arr); err != nil {
			return nil, err
		}
	}

	if err := s.attachOutlookWeather(ctx, out); err != nil {
		return nil, err
	}

	out.RoundForResponse()

	return out, nil
}

func (s *Store) fillOutlookSlot(ctx context.Context, out *model.RouteOutlook, filter store.RouteOutlookFilter, end string, lookbackDays int, p90Arr *sql.NullFloat64) error {
	analysisStart, _, ok := store.TrailingOutlookWindowDays(end, lookbackDays)
	if !ok {
		return fmt.Errorf("parse route outlook max flight date %q", end)
	}

	out.Analysis.Start = analysisStart
	out.Analysis.End = end

	targetMin, ok := store.HHMMToMinutes(filter.DepTime)
	if !ok {
		return nil
	}

	var (
		onTime           int
		delayed          int
		cancelled        int
		diverted         int
		avgArr           sql.NullFloat64
		medianArr        sql.NullFloat64
		avgArrDelayed    sql.NullFloat64
		medianArrDelayed sql.NullFloat64
		avgDep           sql.NullFloat64
		medianDep        sql.NullFloat64
		avgDepDelayed    sql.NullFloat64
		medianDepDelayed sql.NullFloat64
		p90              sql.NullFloat64
	)

	err := s.db.QueryRowContext(ctx, store.QueryRouteOutlook,
		filter.Origin,
		filter.Dest,
		filter.Carrier,
		analysisStart,
		end,
		filter.DayOfWeek,
		targetMin,
		filter.DepTimeWindowMinutes,
	).Scan(
		&out.Analysis.SampleSize,
		&onTime,
		&delayed,
		&cancelled,
		&diverted,
		&avgArr,
		&medianArr,
		&avgArrDelayed,
		&medianArrDelayed,
		&avgDep,
		&medianDep,
		&avgDepDelayed,
		&medianDepDelayed,
		&p90,
	)
	if err != nil {
		return err
	}

	*p90Arr = p90

	if out.Analysis.SampleSize <= 0 {
		return nil
	}

	counted := onTime
	out.OnTimeCount = &counted
	onTimeRate := ratio(onTime, out.Analysis.SampleSize)
	delayRate := ratio(delayed, out.Analysis.SampleSize)
	cancelRate := ratio(cancelled, out.Analysis.SampleSize)
	divertRate := ratio(diverted, out.Analysis.SampleSize)
	out.OnTimeProbability = &onTimeRate
	out.DelayProbability = &delayRate
	out.CancellationProbability = &cancelRate
	out.DiversionProbability = &divertRate
	out.LikelyArrivalDelayMinutes = nullFloatPtr(avgArr)
	out.MedianArrivalDelayMinutes = nullFloatPtr(medianArr)
	out.LikelyArrivalDelayWhenDelayed = nullFloatPtr(avgArrDelayed)
	out.MedianArrivalDelayWhenDelayed = nullFloatPtr(medianArrDelayed)
	out.LikelyDepartureDelayMinutes = nullFloatPtr(avgDep)
	out.MedianDepartureDelayMinutes = nullFloatPtr(medianDep)
	out.LikelyDepartureDelayWhenDelayed = nullFloatPtr(avgDepDelayed)
	out.MedianDepartureDelayWhenDelayed = nullFloatPtr(medianDepDelayed)

	return nil
}

// attachOutlookWeather reads the category rollup for this route and carrier
// over the outlook analysis window. Weekday and departure time are not filters:
// the modifier is not slot-conditioned.
func (s *Store) attachOutlookWeather(ctx context.Context, out *model.RouteOutlook) error {
	query, args := buildRouteWeatherStatsQuery(store.RouteStatsFilter{
		Origin:    out.Filters.Origin,
		Dest:      out.Filters.Dest,
		StartDate: out.Analysis.Start,
		EndDate:   out.Analysis.End,
		Carrier:   out.Filters.Carrier,
	})

	counts, err := s.queryWeatherCategoryCounts(ctx, query, args...)
	if err != nil {
		return err
	}

	out.Weather = store.AssembleRouteOutlookWeather(counts)

	return nil
}

func (s *Store) setRecommendedConnection(ctx context.Context, out *model.RouteOutlook, p90 sql.NullFloat64) error {
	var (
		delay *float64
		mct   *store.AirportConnectionMCT
	)

	if p90.Valid {
		v := p90.Float64
		delay = &v

		loaded, err := s.airportConnectionMCT(ctx, out.Filters.Dest)
		if err != nil {
			return err
		}

		mct = loaded
	}

	out.Connection = store.ConnectionRecommendation(delay, mct)

	return nil
}

func (s *Store) airportConnectionMCT(ctx context.Context, iata string) (*store.AirportConnectionMCT, error) {
	var dd, di, id, ii sql.NullInt64

	err := s.db.QueryRowContext(ctx, store.QueryAirportMCTMinutes, iata).Scan(&dd, &di, &id, &ii)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return &store.AirportConnectionMCT{
		DomesticToDomestic:           nullIntPtr(dd),
		DomesticToInternational:      nullIntPtr(di),
		InternationalToDomestic:      nullIntPtr(id),
		InternationalToInternational: nullIntPtr(ii),
	}, nil
}

func (s *Store) ensureRouteIdentity(ctx context.Context, origin, dest, carrier string) error {
	query, args := routeIdentityQuery(origin, dest, carrier)

	var one int

	err := s.db.QueryRowContext(ctx, query, args...).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("route stats %s-%s: %w", origin, dest, store.ErrNotFound)
	}

	return err
}

func routeIdentityQuery(origin, dest, carrier string) (string, []any) {
	if carrier == "" {
		return store.QueryRouteIdentity, []any{origin, dest}
	}

	return store.QueryRouteCarrierIdentity, []any{origin, dest, carrier}
}

func buildRouteStatsQuery(template string, filter store.RouteStatsFilter) (string, []any) {
	args := []any{filter.Origin, filter.Dest, filter.StartDate, filter.EndDate}
	n := 5

	var extra strings.Builder

	if filter.Carrier != "" {
		fmt.Fprintf(&extra, " AND iata_code_marketing_airline = $%d", n)

		args = append(args, filter.Carrier)
		n++
	}

	if filter.FlightNumber != "" {
		fmt.Fprintf(&extra, " AND flight_number_marketing_airline = $%d", n)

		args = append(args, filter.FlightNumber)
		n++
	}

	if len(filter.DaysOfWeek) > 0 {
		fmt.Fprintf(&extra, " AND day_of_week = ANY($%d)", n)

		args = append(args, filter.DaysOfWeek)
	}

	carrierOnTime := "NULL::json"
	if filter.Carrier == "" {
		carrierOnTime = store.RouteStatsCarrierOnTimeJSON
	}

	query := strings.Replace(template, routeStatsExtraPlaceholder, extra.String(), 1)
	query = strings.Replace(query, routeStatsCarrierOnTimePlaceholder, carrierOnTime, 1)

	return query, args
}

func emptyRouteStats(filter store.RouteStatsFilter) *model.RouteStats {
	return &model.RouteStats{
		Filters: model.Filters{
			Origin:       filter.Origin,
			Dest:         filter.Dest,
			Carrier:      filter.Carrier,
			FlightNumber: filter.FlightNumber,
			DaysOfWeek:   filter.DaysOfWeek,
			StartDate:    filter.StartDate,
			EndDate:      filter.EndDate,
		},
		Analysis: model.Analysis{
			Start: filter.StartDate,
			End:   filter.EndDate,
		},
		DiversionAirports: []model.AirportCount{},
	}
}

func emptyRouteOutlook(filter store.RouteOutlookFilter) *model.RouteOutlook {
	return &model.RouteOutlook{
		Filters: model.Filters{
			Origin:               filter.Origin,
			Dest:                 filter.Dest,
			Carrier:              filter.Carrier,
			DayOfWeek:            filter.DayOfWeek,
			DepTime:              filter.DepTime,
			DepTimeWindowMinutes: filter.DepTimeWindowMinutes,
		},
		OutlookResult: model.OutlookResult{
			Analysis: model.Analysis{
				Confidence: model.ConfidenceUnknown,
			},
		},
	}
}

func unmarshalCarrierOnTime(raw []byte) ([]model.CarrierOnTime, error) {
	if len(raw) == 0 || string(raw) == jsonNull {
		return []model.CarrierOnTime{}, nil
	}

	var rows []struct {
		Carrier string `json:"carrier"`
		OnTime  int    `json:"on_time"`
		Flights int    `json:"flights"`
	}

	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}

	out := make([]model.CarrierOnTime, 0, len(rows))

	for _, row := range rows {
		out = append(out, store.CarrierOnTimeFromCounts(row.Carrier, row.OnTime, row.Flights))
	}

	store.SortCarrierOnTime(out)

	return out, nil
}

func unmarshalAirportCounts(raw []byte) ([]model.AirportCount, error) {
	out := []model.AirportCount{}
	if len(raw) == 0 || string(raw) == jsonNull {
		return out, nil
	}

	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}

	if out == nil {
		return []model.AirportCount{}, nil
	}

	return out, nil
}

func ratio(part, total int) float64 {
	if total == 0 {
		return 0
	}

	return float64(part) / float64(total)
}

func nullFloat(n sql.NullFloat64) float64 {
	if !n.Valid {
		return 0
	}

	return n.Float64
}

func nullFloatPtr(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}

	v := n.Float64

	return &v
}

func nullIntPtr(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}

	v := int(n.Int64)

	return &v
}
