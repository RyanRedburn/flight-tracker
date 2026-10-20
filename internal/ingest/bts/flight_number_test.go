package bts

import (
	"errors"
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/csvparse"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func TestNormalizeFlightNumbers(t *testing.T) {
	row := make([]string, len(DBColumns))
	row[dbColumnIndex(colFlightNumberMarketing)] = "0123"
	row[dbColumnIndex(colFlightNumberOperating)] = " 0007 "
	row[dbColumnIndex(colFlightNumberCodeshare)] = ""

	blank := make([]string, len(DBColumns))
	blank[dbColumnIndex(colFlightNumberMarketing)] = "   "
	blank[dbColumnIndex(colFlightNumberOperating)] = ""
	blank[dbColumnIndex(colFlightNumberCodeshare)] = "0042"

	rows := [][]string{row, blank}
	if err := normalizeFlightNumbers(rows); err != nil {
		t.Fatalf("normalizeFlightNumbers() error = %v", err)
	}

	if row[dbColumnIndex(colFlightNumberMarketing)] != "123" {
		t.Errorf("marketing = %q, want 123", row[dbColumnIndex(colFlightNumberMarketing)])
	}

	if row[dbColumnIndex(colFlightNumberOperating)] != "7" {
		t.Errorf("operating = %q, want 7", row[dbColumnIndex(colFlightNumberOperating)])
	}

	if row[dbColumnIndex(colFlightNumberCodeshare)] != store.MissingFlightNumber {
		t.Errorf("codeshare = %q, want empty", row[dbColumnIndex(colFlightNumberCodeshare)])
	}

	if blank[dbColumnIndex(colFlightNumberMarketing)] != store.MissingFlightNumber ||
		blank[dbColumnIndex(colFlightNumberMarketing)] != row[dbColumnIndex(colFlightNumberCodeshare)] {
		t.Fatal("blank flight numbers must share the empty rollup key")
	}

	if blank[dbColumnIndex(colFlightNumberCodeshare)] != "42" {
		t.Errorf("codeshare = %q, want 42", blank[dbColumnIndex(colFlightNumberCodeshare)])
	}
}

func TestNormalizeFlightNumbersRejectsBadValues(t *testing.T) {
	tests := []struct {
		name   string
		column string
		raw    string
	}{
		{name: "too long", column: colFlightNumberMarketing, raw: "12345"},
		{name: "non digits", column: colFlightNumberOperating, raw: "12A"},
		{name: "all zeros", column: colFlightNumberCodeshare, raw: "0000"},
		{name: "empty is not zero", column: colFlightNumberMarketing, raw: "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := make([]string, len(DBColumns))
			row[dbColumnIndex(tt.column)] = tt.raw

			err := normalizeFlightNumbers([][]string{row})
			if !errors.Is(err, store.ErrInvalidFlightNumber) {
				t.Fatalf("normalizeFlightNumbers() error = %v, want %v", err, store.ErrInvalidFlightNumber)
			}

			if !strings.Contains(err.Error(), tt.column) {
				t.Fatalf("error = %q, want column %s", err.Error(), tt.column)
			}
		})
	}
}

func TestFixtureFlightNumbersNormalize(t *testing.T) {
	file := openFixtureCSV(t)
	defer file.Close()

	_, rows, err := csvparse.Parse(file, DBColumns, csvHeaderToColumn)
	if err != nil {
		t.Fatalf("csvparse.Parse() error = %v", err)
	}

	if err := normalizeFlightNumbers(rows); err != nil {
		t.Fatalf("normalizeFlightNumbers() error = %v", err)
	}

	marketing := rows[0][dbColumnIndex(colFlightNumberMarketing)]
	if marketing != "1" {
		t.Errorf("first marketing flight number = %q, want 1", marketing)
	}
}
