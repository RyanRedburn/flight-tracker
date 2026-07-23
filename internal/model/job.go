package model

import (
	"encoding/json"
	"time"
)

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

const JobTypeImportFlightPerformance = "import_flight_performance"

const JobTypeImportWeatherObservations = "import_weather_observations"

const (
	JobTypeImportCountries = "import_countries"
	JobTypeImportRegions   = "import_regions"
	JobTypeImportAirports  = "import_airports"
)

type Job struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	Status    JobStatus       `json:"status"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	StartedAt *time.Time      `json:"started_at,omitempty"`
	EndedAt   *time.Time      `json:"ended_at,omitempty"`
}

type YearMonth struct {
	Year  int `json:"year"`
	Month int `json:"month"`
}

type FlightPerformanceIngestJob struct {
	JobID string `json:"job_id"`
	Year  int    `json:"year"`
	Month int    `json:"month"`
}

type WeatherIngestJob struct {
	JobID string `json:"job_id"`
	Year  int    `json:"year"`
	Month int    `json:"month"`
}
