package ingest

import (
	"errors"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

func TestExpandMonthsSingleMonth(t *testing.T) {
	months, err := ExpandMonths(RangeInput{StartYear: 2026, StartMonth: 4}, 24)
	if err != nil {
		t.Fatalf("ExpandMonths() error = %v", err)
	}

	if len(months) != 1 {
		t.Fatalf("len(months) = %d, want 1", len(months))
	}

	if months[0] != (model.YearMonth{Year: 2026, Month: 4}) {
		t.Errorf("months[0] = %+v", months[0])
	}
}

func TestExpandMonthsRange(t *testing.T) {
	endYear := 2026
	endMonth := 3

	months, err := ExpandMonths(RangeInput{
		StartYear:  2026,
		StartMonth: 1,
		EndYear:    &endYear,
		EndMonth:   &endMonth,
	}, 24)
	if err != nil {
		t.Fatalf("ExpandMonths() error = %v", err)
	}

	if len(months) != 3 {
		t.Fatalf("len(months) = %d, want 3", len(months))
	}

	want := []model.YearMonth{
		{Year: 2026, Month: 1},
		{Year: 2026, Month: 2},
		{Year: 2026, Month: 3},
	}

	for i, ym := range want {
		if months[i] != ym {
			t.Errorf("months[%d] = %+v, want %+v", i, months[i], ym)
		}
	}
}

func TestExpandMonthsCrossYear(t *testing.T) {
	endYear := 2027
	endMonth := 1

	months, err := ExpandMonths(RangeInput{
		StartYear:  2026,
		StartMonth: 12,
		EndYear:    &endYear,
		EndMonth:   &endMonth,
	}, 24)
	if err != nil {
		t.Fatalf("ExpandMonths() error = %v", err)
	}

	if len(months) != 2 {
		t.Fatalf("len(months) = %d, want 2", len(months))
	}
}

func TestExpandMonthsTooLarge(t *testing.T) {
	endYear := 2026
	endMonth := 4

	_, err := ExpandMonths(RangeInput{
		StartYear:  2026,
		StartMonth: 1,
		EndYear:    &endYear,
		EndMonth:   &endMonth,
	}, 2)
	if !errors.Is(err, ErrRangeTooLarge) {
		t.Fatalf("ExpandMonths() error = %v, want ErrRangeTooLarge", err)
	}
}

func TestExpandMonthsEndBeforeStart(t *testing.T) {
	endYear := 2026
	endMonth := 1

	_, err := ExpandMonths(RangeInput{
		StartYear:  2026,
		StartMonth: 3,
		EndYear:    &endYear,
		EndMonth:   &endMonth,
	}, 24)
	if !errors.Is(err, ErrEndBeforeStart) {
		t.Fatalf("ExpandMonths() error = %v, want ErrEndBeforeStart", err)
	}
}
