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

// MonthIngestConflictResponse is returned when flight-performance or weather ingest
// conflicts with active jobs (active_ingest_months) or existing data (existing_data_months).
type MonthIngestConflictResponse struct {
	Error              string            `json:"error"`
	ActiveIngestMonths []model.YearMonth `json:"active_ingest_months,omitempty"`
	ExistingDataMonths []model.YearMonth `json:"existing_data_months,omitempty"`
}

// QueuedJobResponse is a parameterless job returned after queueing.
type QueuedJobResponse struct {
	ID     string          `json:"id"`
	Type   model.JobType   `json:"type"`
	Status model.JobStatus `json:"status"`
}

// ReferenceIngestConflictResponse is returned when reference-data ingest conflicts
// with an active job (job_type) or existing data (dataset).
type ReferenceIngestConflictResponse struct {
	Error   string        `json:"error"`
	JobType model.JobType `json:"job_type,omitempty"`
	Dataset string        `json:"dataset,omitempty"`
}
