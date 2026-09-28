package operator

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/ingest/mct"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestAirportMCTHandlerType(t *testing.T) {
	h := NewAirportMCTHandler(mct.NewService(&storetest.Stub{}, nil))
	if got := h.Type(); got != model.JobTypeImportAirportMCT {
		t.Errorf("Type() = %q, want %q", got, model.JobTypeImportAirportMCT)
	}
}

func TestAirportMCTHandlerProcess(t *testing.T) {
	minutes := 45
	svc := mct.NewService(&storetest.Stub{
		ReplaceAirportMCTFn: func(context.Context, []string, [][]string) error {
			return nil
		},
	}, nil).WithFetcher(func(context.Context) ([]mct.Airport, error) {
		return []mct.Airport{{IATACode: "SFO", DomesticToDomestic: &minutes}}, nil
	})

	raw, err := NewAirportMCTHandler(svc).Process(context.Background(), &model.Job{ID: testJobID})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	var result mct.ImportResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if result.Dataset != store.ReferenceAirportMCT || result.RowsImported != 1 {
		t.Fatalf("result = %+v", result)
	}
}

func TestAirportMCTHandlerImportError(t *testing.T) {
	h := NewAirportMCTHandler(mct.NewService(&storetest.Stub{}, nil))

	if _, err := h.Process(context.Background(), &model.Job{ID: testJobID}); err == nil {
		t.Fatal("expected import error")
	}
}
