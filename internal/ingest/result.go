package ingest

// MonthImportResult is the JSON result for a BTS or IEM month import.
type MonthImportResult struct {
	Year         int `json:"year"`
	Month        int `json:"month"`
	RowsImported int `json:"rows_imported"`
}
