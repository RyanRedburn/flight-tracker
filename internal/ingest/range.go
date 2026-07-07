package ingest

import (
	"errors"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

var (
	ErrEndBeforeStart  = errors.New("end date must be on or after start date")
	ErrRangeTooLarge   = errors.New("requested range exceeds maximum allowed months")
	ErrMaxIngestMonths = errors.New("max ingest months must be >= 1")
)

type RangeInput struct {
	StartYear  int
	StartMonth int
	EndYear    *int
	EndMonth   *int
}

func ExpandMonths(in RangeInput, maxMonths int) ([]model.YearMonth, error) {
	if maxMonths < 1 {
		return nil, ErrMaxIngestMonths
	}

	endYear, endMonth, err := resolveEnd(in)
	if err != nil {
		return nil, err
	}

	if yearMonthBefore(endYear, endMonth, in.StartYear, in.StartMonth) {
		return nil, ErrEndBeforeStart
	}

	months := make([]model.YearMonth, 0, maxMonths)
	year, month := in.StartYear, in.StartMonth

	for {
		months = append(months, model.YearMonth{Year: year, Month: month})

		if year == endYear && month == endMonth {
			break
		}

		if len(months) > maxMonths {
			return nil, ErrRangeTooLarge
		}

		month++
		if month > 12 {
			month = 1
			year++
		}
	}

	return months, nil
}

func resolveEnd(in RangeInput) (int, int, error) {
	if in.EndYear == nil && in.EndMonth == nil {
		return in.StartYear, in.StartMonth, nil
	}

	if in.EndYear == nil || in.EndMonth == nil {
		return 0, 0, errors.New("end_year and end_month must both be set or both omitted")
	}

	return *in.EndYear, *in.EndMonth, nil
}

func yearMonthBefore(y1, m1, y2, m2 int) bool {
	return y1 < y2 || (y1 == y2 && m1 < m2)
}
