package store

import (
	"sort"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	DefaultCarrierStatsSpanDays = 90
	// CarrierRankMinSampleSize is the minimum flights for a route or airport to be ranked.
	CarrierRankMinSampleSize = 30
	CarrierStatsTopN         = 5
)

type CarrierStatsFilter struct {
	Carrier   string
	StartDate string
	EndDate   string
	State     string
}

type carrierRankKey struct {
	rate    float64
	flights int
	first   string
	second  string
}

func CarrierStatsWindow(endDate string) (start, end string, ok bool) {
	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return "", "", false
	}

	return maxCarrierWindowStart(endTime), endDate, true
}

func RankCarrierRoutes(items []model.CarrierRouteStat) ([]model.CarrierRouteStat, []model.CarrierRouteStat) {
	best, worst := rankCarrier(items, func(item model.CarrierRouteStat) carrierRankKey {
		return carrierRankKey{
			rate:    item.OnTimeRate,
			flights: item.Flights,
			first:   item.Origin,
			second:  item.Dest,
		}
	})

	return nonNilRoutes(best), nonNilRoutes(worst)
}

func RankCarrierAirports(items []model.CarrierAirportStat) ([]model.CarrierAirportStat, []model.CarrierAirportStat) {
	best, worst := rankCarrier(items, func(item model.CarrierAirportStat) carrierRankKey {
		return carrierRankKey{
			rate:    item.OnTimeRate,
			flights: item.Flights,
			first:   item.Airport,
		}
	})

	return nonNilAirports(best), nonNilAirports(worst)
}

func rankCarrier[T any](items []T, key func(T) carrierRankKey) ([]T, []T) {
	qualifying := make([]T, 0, len(items))
	for _, item := range items {
		if key(item).flights >= CarrierRankMinSampleSize {
			qualifying = append(qualifying, item)
		}
	}

	sort.Slice(qualifying, func(i, j int) bool {
		return carrierRankLess(key(qualifying[i]), key(qualifying[j]), true)
	})

	best, rest := splitTopN(qualifying, CarrierStatsTopN)
	sort.Slice(rest, func(i, j int) bool {
		return carrierRankLess(key(rest[i]), key(rest[j]), false)
	})

	worst, _ := splitTopN(rest, CarrierStatsTopN)

	return best, worst
}

func carrierRankLess(a, b carrierRankKey, best bool) bool {
	if a.rate != b.rate {
		if best {
			return a.rate > b.rate
		}

		return a.rate < b.rate
	}

	if a.flights != b.flights {
		return a.flights > b.flights
	}

	if a.first != b.first {
		return a.first < b.first
	}

	return a.second < b.second
}

func splitTopN[T any](items []T, n int) (top, rest []T) {
	if len(items) == 0 {
		return nil, nil
	}

	if len(items) <= n {
		return append([]T(nil), items...), nil
	}

	return append([]T(nil), items[:n]...), append([]T(nil), items[n:]...)
}

func nonNilRoutes(items []model.CarrierRouteStat) []model.CarrierRouteStat {
	if items == nil {
		return []model.CarrierRouteStat{}
	}

	return items
}

func nonNilAirports(items []model.CarrierAirportStat) []model.CarrierAirportStat {
	if items == nil {
		return []model.CarrierAirportStat{}
	}

	return items
}

func maxCarrierWindowStart(end time.Time) string {
	return end.AddDate(0, 0, -(DefaultCarrierStatsSpanDays - 1)).Format("2006-01-02")
}
