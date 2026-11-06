package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/domain"
	"github.com/RyanRedburn/flight-tracker/internal/model"
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

// Validate checks the weekday, departure time, and departure window.
func (f RouteOutlookFilter) Validate() error {
	if f.DayOfWeek < 1 || f.DayOfWeek > 7 {
		return domain.ErrInvalidWeekday
	}

	if _, ok := model.HHMMToMinutes(f.DepTime); !ok {
		return domain.ErrInvalidHHMM
	}

	return domain.DepTimeWindowError(f.DepTimeWindowMinutes)
}

// Validate checks the stats dates, the inclusive span, and each weekday.
func (f RouteStatsFilter) Validate() error {
	start, err := time.Parse("2006-01-02", strings.TrimSpace(f.StartDate))
	if err != nil {
		return domain.ErrInvalidDate
	}

	end, err := time.Parse("2006-01-02", strings.TrimSpace(f.EndDate))
	if err != nil {
		return domain.ErrInvalidDate
	}

	if end.Before(start) {
		return domain.ErrStatsDateOrder
	}

	if domain.ExceedsMaxStatsSpan(start, end) {
		return fmt.Errorf("date range must be at most %d days", domain.MaxStatsSpanDays)
	}

	for _, day := range f.DaysOfWeek {
		if day < 1 || day > 7 {
			return domain.ErrInvalidWeekday
		}
	}

	return nil
}
