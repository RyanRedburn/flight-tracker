package handlers

import "github.com/RyanRedburn/flight-tracker/internal/model"

// ErrorResponse is the common JSON error body returned by the API.
type ErrorResponse struct {
	Error string `json:"error"`
}

// StatusResponse is a simple status payload used by health endpoints.
type StatusResponse struct {
	Status string `json:"status"`
}

// FlightPerformanceIngestConflictResponse is returned when flight-performance ingest conflicts
// with active jobs (active_ingest_months) or existing data (existing_data_months).
type FlightPerformanceIngestConflictResponse struct {
	Error              string            `json:"error"`
	ActiveIngestMonths []model.YearMonth `json:"active_ingest_months,omitempty"`
	ExistingDataMonths []model.YearMonth `json:"existing_data_months,omitempty"`
}

// WeatherIngestConflictResponse is returned when weather ingest conflicts
// with active jobs (active_ingest_months) or existing data (existing_data_months).
type WeatherIngestConflictResponse struct {
	Error              string            `json:"error"`
	ActiveIngestMonths []model.YearMonth `json:"active_ingest_months,omitempty"`
	ExistingDataMonths []model.YearMonth `json:"existing_data_months,omitempty"`
}

// ReferenceIngestConflictResponse is returned when reference-data ingest conflicts
// with an active job (job_type) or existing data (dataset).
type ReferenceIngestConflictResponse struct {
	Error   string `json:"error"`
	JobType string `json:"job_type,omitempty"`
	Dataset string `json:"dataset,omitempty"`
}
