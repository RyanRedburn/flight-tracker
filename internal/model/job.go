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

type JobType string

const (
	JobTypeImportFlightPerformance   JobType = "import_flight_performance"
	JobTypeImportWeatherObservations JobType = "import_weather_observations"
	JobTypeImportCountries           JobType = "import_countries"
	JobTypeImportRegions             JobType = "import_regions"
	JobTypeImportAirports            JobType = "import_airports"
	JobTypeImportWeatherStations     JobType = "import_weather_stations"
	JobTypeRebuildRouteTravelWindows JobType = "rebuild_route_travel_windows"
	JobTypeRebuildRouteWeatherStats  JobType = "rebuild_route_weather_stats"
)

type Job struct {
	ID        string          `json:"id"`
	Type      JobType         `json:"type"`
	Status    JobStatus       `json:"status"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	StartedAt *time.Time      `json:"started_at,omitempty"`
	EndedAt   *time.Time      `json:"ended_at,omitempty"`

	// ListIngest is set by ListJobs. Nil when the job was loaded without that join.
	ListIngest *JobListIngest `json:"-"`
}

// JobListIngest is year, month, and weather stations joined onto a jobs list row.
// Year and month are nil when the job has no ingest detail row.
type JobListIngest struct {
	Year     *int
	Month    *int
	Stations []string
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
	JobID    string   `json:"job_id"`
	Year     int      `json:"year"`
	Month    int      `json:"month"`
	Stations []string `json:"stations"`
}
