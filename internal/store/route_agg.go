package store

import (
	"sort"
	"strconv"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	MinOutlookSampleSize        = 10
	OutlookLookbackDays         = 365
	MaxStatsSpanDays            = 366
	DefaultDepTimeWindowMinutes = 30
	MaxDepTimeWindowMinutes     = 120
)

type RouteStatsFilter struct {
	Origin       string
	Dest         string
	StartDate    string
	EndDate      string
	Carrier      string
	FlightNumber string
	DaysOfWeek   []int
}

type RouteOutlookFilter struct {
	Origin               string
	Dest                 string
	Carrier              string
	DayOfWeek            int
	DepTime              string
	DepTimeWindowMinutes int
}

func HHMMToMinutes(hhmm string) (int, bool) {
	hhmm = strings.TrimSpace(hhmm)
	if hhmm == "" {
		return 0, false
	}
	// Accept 1–4 digit hhmm (e.g. "559", "0559", "700").
	n, err := strconv.Atoi(hhmm)
	if err != nil || n < 0 || n > 2359 {
		return 0, false
	}

	h := n / 100

	m := n % 100
	if h > 23 || m > 59 {
		return 0, false
	}

	return h*60 + m, true
}

func CarrierOnTimeRateFromCounts(carrier string, flights, onTime, delayed, cancelled, diverted int) model.CarrierOnTimeRate {
	return model.CarrierOnTimeRate{
		Carrier:          carrier,
		Flights:          flights,
		OnTime:           onTime,
		Delayed:          delayed,
		Cancelled:        cancelled,
		Diverted:         diverted,
		OnTimeRate:       rate(onTime, flights),
		DelayRate:        rate(delayed, flights),
		CancellationRate: rate(cancelled, flights),
		DiversionRate:    rate(diverted, flights),
	}
}

func SortCarrierOnTimeRates(items []model.CarrierOnTimeRate) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].OnTimeRate != items[j].OnTimeRate {
			return items[i].OnTimeRate > items[j].OnTimeRate
		}

		if items[i].Flights != items[j].Flights {
			return items[i].Flights > items[j].Flights
		}

		return items[i].Carrier < items[j].Carrier
	})
}

func DelayCausesShareFromCounts(delayed, carrier, weather, nas, security, late, unattributed int) model.DelayCausesShare {
	if delayed == 0 {
		return model.DelayCausesShare{}
	}

	return model.DelayCausesShare{
		Carrier:      rate(carrier, delayed),
		Weather:      rate(weather, delayed),
		NAS:          rate(nas, delayed),
		Security:     rate(security, delayed),
		LateAircraft: rate(late, delayed),
		Unattributed: rate(unattributed, delayed),
	}
}

func rate(part, total int) float64 {
	if total == 0 {
		return 0
	}

	return float64(part) / float64(total)
}
