package ingest

import (
	"encoding/json"
	"testing"
)

func TestMonthImportResultJSON(t *testing.T) {
	payload, err := json.Marshal(MonthImportResult{Year: 2026, Month: 4, RowsImported: 42})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded["year"] != float64(2026) || decoded["month"] != float64(4) || decoded["rows_imported"] != float64(42) {
		t.Errorf("result = %v", decoded)
	}
}

func TestDatasetImportResultJSON(t *testing.T) {
	payload, err := json.Marshal(DatasetImportResult{Dataset: "airports", RowsImported: 42})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	const want = `{"dataset":"airports","rows_imported":42}`
	if string(payload) != want {
		t.Errorf("json = %s, want %s", payload, want)
	}
}
