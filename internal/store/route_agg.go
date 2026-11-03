package store

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	OutlookLookbackDays         = 365
	OutlookFallbackLookbackDays = 730
	MaxStatsSpanDays            = 366
	DefaultDepTimeWindowMinutes = 60
	MaxDepTimeWindowMinutes     = 120
)

var (
	ErrOutlookSchedule = errors.New("exactly one of date or day_of_week is required")
	ErrInvalidDate     = errors.New("date must be a valid date (YYYY-MM-DD)")
	ErrInvalidWeekday  = errors.New("day_of_week must be from 1 to 7")
	ErrInvalidHHMM     = errors.New("dep_time must be a valid local time (hhmm)")
	ErrStatsDateOrder  = errors.New("end_date must be on or after start_date")
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
	// DepTimeWindowSet is true when the caller passed a window.
	// The default window may widen lookback. An explicit window does not.
	DepTimeWindowSet bool
}

// ParseOutlookDay resolves an outlook weekday. Exactly one of date or dayOfWeek must be set.
func ParseOutlookDay(date string, dayOfWeek int) (int, error) {
	date = strings.TrimSpace(date)
	hasDate := date != ""
	hasDay := dayOfWeek != 0

	if hasDate == hasDay {
		return 0, ErrOutlookSchedule
	}

	if hasDay {
		if dayOfWeek < 1 || dayOfWeek > 7 {
			return 0, ErrInvalidWeekday
		}

		return dayOfWeek, nil
	}

	day, ok := ISOWeekdayFromDate(date)
	if !ok {
		return 0, ErrInvalidDate
	}

	return day, nil
}

// Validate checks the weekday, departure time, and departure window.
func (f RouteOutlookFilter) Validate() error {
	if f.DayOfWeek < 1 || f.DayOfWeek > 7 {
		return ErrInvalidWeekday
	}

	if _, ok := HHMMToMinutes(f.DepTime); !ok {
		return ErrInvalidHHMM
	}

	return DepTimeWindowError(f.DepTimeWindowMinutes)
}

// DepTimeWindowError reports a window outside 1–MaxDepTimeWindowMinutes.
func DepTimeWindowError(minutes int) error {
	if minutes >= 1 && minutes <= MaxDepTimeWindowMinutes {
		return nil
	}

	return fmt.Errorf("dep_time_window_minutes must be from 1 to %d", MaxDepTimeWindowMinutes)
}

// Validate checks the stats dates, the inclusive span, and each weekday.
func (f RouteStatsFilter) Validate() error {
	start, err := time.Parse("2006-01-02", strings.TrimSpace(f.StartDate))
	if err != nil {
		return ErrInvalidDate
	}

	end, err := time.Parse("2006-01-02", strings.TrimSpace(f.EndDate))
	if err != nil {
		return ErrInvalidDate
	}

	if end.Before(start) {
		return ErrStatsDateOrder
	}

	if ExceedsMaxStatsSpan(start, end) {
		return fmt.Errorf("date range must be at most %d days", MaxStatsSpanDays)
	}

	for _, day := range f.DaysOfWeek {
		if day < 1 || day > 7 {
			return ErrInvalidWeekday
		}
	}

	return nil
}

// ExceedsMaxStatsSpan reports whether the inclusive range is longer than MaxStatsSpanDays.
// A zero or reversed range does not.
func ExceedsMaxStatsSpan(start, end time.Time) bool {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return false
	}

	return int(end.Sub(start).Hours()/24)+1 > MaxStatsSpanDays
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

func MinutesToHHMM(minutes int) string {
	if minutes < 0 {
		minutes = 0
	}

	minutes %= 24 * 60

	return fmt.Sprintf("%02d%02d", minutes/60, minutes%60)
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

// TrailingOutlookWindow is the inclusive trailing outlook year ending at endDate.
// Route outlook and compare_carriers share it; the span fits in MaxStatsSpanDays.
func TrailingOutlookWindow(endDate string) (start, end string, ok bool) {
	return TrailingOutlookWindowDays(endDate, OutlookLookbackDays)
}

// TrailingOutlookWindowDays is the inclusive trailing window of days ending at endDate.
func TrailingOutlookWindowDays(endDate string, days int) (start, end string, ok bool) {
	endTime, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return "", "", false
	}

	return endTime.AddDate(0, 0, -days).Format("2006-01-02"), endDate, true
}

// WidenOutlookLookback reports whether a default-window slot below the low outlook band
// should be re-queried at OutlookFallbackLookbackDays. A caller-set departure window is never widened.
func WidenOutlookLookback(sampleSize int, windowSet bool) bool {
	return !windowSet && sampleSize < model.OutlookLowSampleBelow
}

// ApplyOutlookLookback runs the slot query at 365 days, and at 730 when the default window is still thin.
// It records lookback_days and then applies the outlook sample rule.
func ApplyOutlookLookback(out *model.RouteOutlook, windowSet bool, query func(lookbackDays int) error) error {
	if out == nil {
		return nil
	}

	if err := query(OutlookLookbackDays); err != nil {
		return err
	}

	lookback := OutlookLookbackDays

	if WidenOutlookLookback(out.Analysis.SampleSize, windowSet) {
		if err := query(OutlookFallbackLookbackDays); err != nil {
			return err
		}

		lookback = OutlookFallbackLookbackDays
	}

	out.Analysis.LookbackDays = lookback
	out.ApplyOutlookSample()

	return nil
}

func ResolveDepTimeWindowMinutes(minutes *int) int {
	if minutes == nil {
		return DefaultDepTimeWindowMinutes
	}

	return *minutes
}

func CarrierOnTimeFromCounts(carrier string, onTime, flights int) model.CarrierOnTime {
	return model.CarrierOnTime{
		Carrier:     carrier,
		OnTimeRate:  rate(onTime, flights),
		Flights:     flights,
		OnTimeCount: &onTime,
	}
}

// CarrierComparison is ranked when the carrier with the highest 95% Wilson lower bound
// and at least one flight differs at 95% from another. Fewer than two such carriers is not a comparison.
func CarrierComparison(items []model.CarrierOnTime) string {
	eligible := make([]model.CarrierOnTime, 0, len(items))

	for _, item := range items {
		if item.Flights >= 1 {
			eligible = append(eligible, item)
		}
	}

	if len(eligible) < 2 {
		return ""
	}

	sort.SliceStable(eligible, func(i, j int) bool {
		left, _ := model.WilsonBounds(eligible[i].OnTimeRate, eligible[i].OnTimeCount, eligible[i].Flights)
		right, _ := model.WilsonBounds(eligible[j].OnTimeRate, eligible[j].OnTimeCount, eligible[j].Flights)

		if left != right {
			return left > right
		}

		if eligible[i].Flights != eligible[j].Flights {
			return eligible[i].Flights > eligible[j].Flights
		}

		return eligible[i].Carrier < eligible[j].Carrier
	})

	leader := eligible[0]

	for _, item := range eligible[1:] {
		if carrierDiffers(leader, item) {
			return model.ComparisonRanked
		}
	}

	return model.ComparisonTooCloseToCall
}

func carrierDiffers(a, b model.CarrierOnTime) bool {
	return model.ProportionsDiffer(
		model.ComparisonRate(a.OnTimeRate, a.OnTimeCount, a.Flights),
		a.Flights,
		model.ComparisonRate(b.OnTimeRate, b.OnTimeCount, b.Flights),
		b.Flights,
	)
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
