package store

import (
	"sort"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	DefaultCarrierStatsSpanDays = 90
	CarrierStatsMinSampleSize   = 30
	CarrierStatsTopN            = 5
)

type CarrierStatsFilter struct {
	Carrier   string
	StartDate string
	EndDate   string
	State     string
}

func CarrierStatsWindow(endDate string) (start, end string, ok bool) {
	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return "", "", false
	}

	return maxCarrierWindowStart(endTime), endDate, true
}

func RankCarrierRoutes(items []model.CarrierRouteStat) ([]model.CarrierRouteStat, []model.CarrierRouteStat) {
	qualifying := make([]model.CarrierRouteStat, 0, len(items))
	for _, item := range items {
		if item.Flights >= CarrierStatsMinSampleSize {
			qualifying = append(qualifying, item)
		}
	}

	sort.Slice(qualifying, func(i, j int) bool {
		if qualifying[i].OnTimeRate != qualifying[j].OnTimeRate {
			return qualifying[i].OnTimeRate > qualifying[j].OnTimeRate
		}

		if qualifying[i].Flights != qualifying[j].Flights {
			return qualifying[i].Flights > qualifying[j].Flights
		}

		if qualifying[i].Origin != qualifying[j].Origin {
			return qualifying[i].Origin < qualifying[j].Origin
		}

		return qualifying[i].Dest < qualifying[j].Dest
	})

	best, rest := splitTopN(qualifying, CarrierStatsTopN)
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].OnTimeRate != rest[j].OnTimeRate {
			return rest[i].OnTimeRate < rest[j].OnTimeRate
		}

		if rest[i].Flights != rest[j].Flights {
			return rest[i].Flights > rest[j].Flights
		}

		if rest[i].Origin != rest[j].Origin {
			return rest[i].Origin < rest[j].Origin
		}

		return rest[i].Dest < rest[j].Dest
	})

	worst, _ := splitTopN(rest, CarrierStatsTopN)

	return nonNilRoutes(best), nonNilRoutes(worst)
}

func RankCarrierAirports(items []model.CarrierAirportStat) ([]model.CarrierAirportStat, []model.CarrierAirportStat) {
	qualifying := make([]model.CarrierAirportStat, 0, len(items))
	for _, item := range items {
		if item.Flights >= CarrierStatsMinSampleSize {
			qualifying = append(qualifying, item)
		}
	}

	sort.Slice(qualifying, func(i, j int) bool {
		if qualifying[i].OnTimeRate != qualifying[j].OnTimeRate {
			return qualifying[i].OnTimeRate > qualifying[j].OnTimeRate
		}

		if qualifying[i].Flights != qualifying[j].Flights {
			return qualifying[i].Flights > qualifying[j].Flights
		}

		return qualifying[i].Airport < qualifying[j].Airport
	})

	best, rest := splitTopN(qualifying, CarrierStatsTopN)
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].OnTimeRate != rest[j].OnTimeRate {
			return rest[i].OnTimeRate < rest[j].OnTimeRate
		}

		if rest[i].Flights != rest[j].Flights {
			return rest[i].Flights > rest[j].Flights
		}

		return rest[i].Airport < rest[j].Airport
	})

	worst, _ := splitTopN(rest, CarrierStatsTopN)

	return nonNilAirports(best), nonNilAirports(worst)
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
