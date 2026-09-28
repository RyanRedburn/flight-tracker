package mct

import (
	"context"
	"errors"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestServiceImport(t *testing.T) {
	var gotColumns []string

	var gotRows [][]string

	st := &storetest.Stub{
		ReplaceAirportMCTFn: func(_ context.Context, columns []string, rows [][]string) error {
			gotColumns = columns
			gotRows = rows

			return nil
		},
	}

	domestic := 60
	svc := NewService(st, nil).WithFetcher(func(context.Context) ([]Airport, error) {
		return []Airport{
			{IATACode: testIATAATL, Name: "Atlanta", DomesticToDomestic: &domestic},
			{Code: "nope", Name: "skip"},
		}, nil
	})

	result, err := svc.Import(context.Background())
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}

	if result.Dataset != store.ReferenceAirportMCT || result.RowsImported != 1 {
		t.Fatalf("result = %+v", result)
	}

	if len(gotColumns) != len(airportMCTColumns) || len(gotRows) != 1 || gotRows[0][0] != testIATAATL || gotRows[0][8] != "60" {
		t.Fatalf("replace columns=%v rows=%v", gotColumns, gotRows)
	}
}

func TestServiceImportWithoutFetcher(t *testing.T) {
	svc := NewService(&storetest.Stub{}, nil)

	_, err := svc.Import(context.Background())
	if err == nil {
		t.Fatal("Import() expected error without fetcher")
	}
}

func TestServiceImportDoesNotReplaceOnFetchError(t *testing.T) {
	svc := NewService(&storetest.Stub{}, nil).WithFetcher(func(context.Context) ([]Airport, error) {
		return nil, errors.New("upstream down")
	})

	_, err := svc.Import(context.Background())
	if err == nil {
		t.Fatal("Import() expected fetch error")
	}
}

func TestServiceImportEmptyCatalog(t *testing.T) {
	svc := NewService(&storetest.Stub{}, nil).WithFetcher(func(context.Context) ([]Airport, error) {
		return []Airport{{Code: "x"}}, nil
	})

	_, err := svc.Import(context.Background())
	if err == nil {
		t.Fatal("Import() expected empty catalog error")
	}
}
