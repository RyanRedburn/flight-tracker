package store

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

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

func FormatHHMM(raw string) string {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return raw
	}

	return fmt.Sprintf("%04d", n)
}

// ISOWeekday maps a calendar date to 1=Monday … 7=Sunday.
func ISOWeekday(t time.Time) int {
	if t.Weekday() == time.Sunday {
		return 7
	}

	return int(t.Weekday())
}

func ISOWeekdayFromDate(date string) (int, bool) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(date))
	if err != nil {
		return 0, false
	}

	return ISOWeekday(t), true
}

func ResolveDepTimeWindowMinutes(minutes *int) int {
	if minutes == nil {
		return DefaultDepTimeWindowMinutes
	}

	return *minutes
}

func CarrierOnTimeFromCounts(carrier string, onTime, flights int) model.CarrierOnTime {
	return model.CarrierOnTime{
		Carrier:    carrier,
		OnTimeRate: rate(onTime, flights),
		Flights:    flights,
	}
}

func SortCarrierOnTime(items []model.CarrierOnTime) {
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
