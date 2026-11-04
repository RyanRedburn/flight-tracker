package ingest

import "github.com/RyanRedburn/flight-tracker/internal/store"

// MonthImportResult is the JSON result for a BTS or IEM month import.
type MonthImportResult struct {
	Year         int `json:"year"`
	Month        int `json:"month"`
	RowsImported int `json:"rows_imported"`
}

// DatasetImportResult is the JSON result for a full-table import.
type DatasetImportResult struct {
	Dataset      store.ReferenceDataset `json:"dataset"`
	RowsImported int                    `json:"rows_imported"`
}
