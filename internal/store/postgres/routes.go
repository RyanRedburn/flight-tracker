package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const routeStatsExtraPlaceholder = "/*extra*/"

func (s *Store) RouteStats(ctx context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error) {
	stats := emptyRouteStats(filter)

	query, args := buildRouteStatsQuery(filter)

	var (
		avgArr           sql.NullFloat64
		medianArr        sql.NullFloat64
		avgArrDelayed    sql.NullFloat64
		medianArrDelayed sql.NullFloat64
		avgDep           sql.NullFloat64
		avgDepDelayed    sql.NullFloat64
		causeCarrier     sql.NullFloat64
		causeWeather     sql.NullFloat64
		causeNAS         sql.NullFloat64
		causeSecurity    sql.NullFloat64
		causeLate        sql.NullFloat64
		diversionJSON    []byte
		cancelJSON       []byte
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
		&diversionJSON,
		&cancelJSON,
	)
	if err != nil {
		return nil, err
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
	}

	airports, err := unmarshalAirportCounts(diversionJSON)
	if err != nil {
		return nil, fmt.Errorf("decode diversion airports: %w", err)
	}

	codes, err := unmarshalCancellationCodes(cancelJSON)
	if err != nil {
		return nil, fmt.Errorf("decode cancellation codes: %w", err)
	}

	stats.DiversionAirports = airports
	stats.CancellationCodes = codes

	stats.RoundForResponse()

	return stats, nil
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
		return out, nil
	}

	endTime, err := time.Parse("2006-01-02", analysisEnd.String)
	if err != nil {
		return out, nil
	}

	analysisStart := endTime.AddDate(0, 0, -store.OutlookLookbackDays).Format("2006-01-02")
	out.AnalysisStart = analysisStart
	out.AnalysisEnd = analysisEnd.String

	targetMin, ok := store.HHMMToMinutes(filter.DepTime)
	if !ok {
		return out, nil
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
	)

	err = s.db.QueryRowContext(ctx, store.QueryRouteOutlook,
		filter.Origin,
		filter.Dest,
		filter.Carrier,
		analysisStart,
		analysisEnd.String,
		filter.DayOfWeek,
		targetMin,
		filter.DepTimeWindowMinutes,
	).Scan(
		&out.SampleSize,
		&onTime,
		&delayed,
		&cancelled,
		&diverted,
		&avgArr,
		&medianArr,
		&avgArrDelayed,
		&medianArrDelayed,
		&avgDep,
	)
	if err != nil {
		return nil, err
	}

	out.InsufficientSample = out.SampleSize > 0 && out.SampleSize < store.MinOutlookSampleSize
	if out.SampleSize == 0 {
		return out, nil
	}

	out.OnTimeProbability = ratio(onTime, out.SampleSize)
	out.DelayProbability = ratio(delayed, out.SampleSize)
	out.CancellationProbability = ratio(cancelled, out.SampleSize)
	out.DiversionProbability = ratio(diverted, out.SampleSize)
	out.LikelyArrivalDelayMinutes = nullFloat(avgArr)
	out.MedianArrivalDelayMinutes = nullFloat(medianArr)
	out.LikelyArrivalDelayWhenDelayed = nullFloat(avgArrDelayed)
	out.MedianArrivalDelayWhenDelayed = nullFloat(medianArrDelayed)
	out.LikelyDepartureDelayMinutes = nullFloat(avgDep)

	out.RoundForResponse()

	return out, nil
}

func buildRouteStatsQuery(filter store.RouteStatsFilter) (string, []any) {
	args := []any{filter.Origin, filter.Dest, filter.StartDate, filter.EndDate}
	n := 5

	var extra strings.Builder

	if filter.Carrier != "" {
		fmt.Fprintf(&extra, " AND iata_code_marketing_airline = $%d", n)

		args = append(args, filter.Carrier)
		n++
	}

	if filter.FlightNumber != "" {
		fmt.Fprintf(&extra, " AND flight_number_marketing_airline::text = $%d", n)

		args = append(args, filter.FlightNumber)
		n++
	}

	if len(filter.DaysOfWeek) > 0 {
		fmt.Fprintf(&extra, " AND day_of_week = ANY($%d)", n)

		args = append(args, filter.DaysOfWeek)
	}

	query := strings.Replace(store.QueryRouteStats, routeStatsExtraPlaceholder, extra.String(), 1)

	return query, args
}

func emptyRouteStats(filter store.RouteStatsFilter) *model.RouteStats {
	days := append([]int(nil), filter.DaysOfWeek...)
	if days == nil {
		days = []int{}
	}

	return &model.RouteStats{
		Origin:    filter.Origin,
		Dest:      filter.Dest,
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
		Filters: model.RouteStatsFilters{
			Carrier:      filter.Carrier,
			FlightNumber: filter.FlightNumber,
			DaysOfWeek:   days,
		},
		DiversionAirports: []model.AirportCount{},
		CancellationCodes: []model.CancellationCodeCount{},
	}
}

func emptyRouteOutlook(filter store.RouteOutlookFilter) *model.RouteOutlook {
	return &model.RouteOutlook{
		Origin:               filter.Origin,
		Dest:                 filter.Dest,
		Carrier:              filter.Carrier,
		DayOfWeek:            filter.DayOfWeek,
		DepTime:              filter.DepTime,
		DepTimeWindowMinutes: filter.DepTimeWindowMinutes,
	}
}

func unmarshalAirportCounts(raw []byte) ([]model.AirportCount, error) {
	out := []model.AirportCount{}
	if len(raw) == 0 || string(raw) == "null" {
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

func unmarshalCancellationCodes(raw []byte) ([]model.CancellationCodeCount, error) {
	out := []model.CancellationCodeCount{}
	if len(raw) == 0 || string(raw) == "null" {
		return out, nil
	}

	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}

	if out == nil {
		return []model.CancellationCodeCount{}, nil
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
