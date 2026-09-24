package postgres

import (
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func TestExpandMonthsIn(t *testing.T) {
	query, args := expandMonthsIn(store.QueryMonthsWithFlightPerformanceData, []model.YearMonth{
		{Year: 2024, Month: 1},
		{Year: 2024, Month: 2},
	})

	if strings.Contains(query, monthsInPlaceholder) {
		t.Fatal("placeholder should be replaced")
	}

	if !strings.Contains(query, "($1, $2), ($3, $4)") {
		t.Fatalf("query = %s", query)
	}

	if len(args) != 4 || args[0] != 2024 || args[1] != 1 || args[2] != 2024 || args[3] != 2 {
		t.Fatalf("args = %#v", args)
	}
}
