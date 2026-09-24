package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

var (
	ErrNotFound             = errors.New("not found")
	ErrJobStatusConflict    = errors.New("job status conflict")
	ErrActiveIngestConflict = errors.New("active ingest job conflict")
	ErrConflict             = errors.New("conflict")
)

type MigrationVersion struct {
	Version uint `json:"version"`
	Dirty   bool `json:"dirty"`
}

type Store interface {
	CreateJob(ctx context.Context, job *model.Job) error
	CreateFlightPerformanceIngestJob(ctx context.Context, year, month int) (*model.Job, error)
	CreateWeatherIngestJob(ctx context.Context, year, month int, stations []string) (*model.Job, error)
	GetJob(ctx context.Context, id string) (*model.Job, error)
	GetFlightPerformanceIngestJob(ctx context.Context, jobID string) (*model.FlightPerformanceIngestJob, error)
	GetWeatherIngestJob(ctx context.Context, jobID string) (*model.WeatherIngestJob, error)
	ListJobs(ctx context.Context, limit int) ([]*model.Job, error)
	UpdateJob(ctx context.Context, job *model.Job) error
	ClaimNextPendingJob(ctx context.Context, leaseUntil time.Time) (*model.Job, error)
	CompleteJob(ctx context.Context, id string, result json.RawMessage) error
	FailJob(ctx context.Context, id, errMsg string) error
	HeartbeatJob(ctx context.Context, id string, leaseUntil time.Time) error
	ResetStaleRunningJobs(ctx context.Context, expiredBefore time.Time) (int64, error)
	ActiveFlightPerformanceIngestMonths(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ActiveWeatherIngestMonths(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ActiveIngestJob(ctx context.Context, jobType string) (bool, error)
	CreateReferenceIngestJob(ctx context.Context, jobType string) (*model.Job, error)
	CreateRebuildRouteTravelWindowsJob(ctx context.Context) (*model.Job, error)
	HasReferenceData(ctx context.Context, dataset ReferenceDataset) (bool, error)
	ReplaceCountries(ctx context.Context, columns []string, rows [][]string) error
	ReplaceRegions(ctx context.Context, columns []string, rows [][]string) error
	ReplaceAirports(ctx context.Context, columns []string, rows [][]string) error
	HasWeatherStationsData(ctx context.Context) (bool, error)
	ReplaceWeatherStations(ctx context.Context, stationColumns []string, stationRows [][]string, mappingColumns []string, mappingRows [][]string) error
	ListAirportWeatherStations(ctx context.Context) ([]AirportWeatherStation, error)
	ListAirportIdentifiersByIATA(ctx context.Context, codes []string) (map[string]AirportIdentifiers, error)
	MonthsWithFlightPerformanceData(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	MonthsWithWeatherData(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	DistinctFlightAirportCodes(ctx context.Context) ([]string, error)
	ReplaceFlightPerformanceByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error
	ReplaceWeatherObservationsByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error
	RouteStats(ctx context.Context, filter RouteStatsFilter) (*model.RouteStats, error)
	RouteOutlook(ctx context.Context, filter RouteOutlookFilter) (*model.RouteOutlook, error)
	RouteTravelWindows(ctx context.Context, filter RouteTravelWindowsFilter) (*model.RouteTravelWindows, error)
	RebuildRouteTravelWindows(ctx context.Context) error
	CarrierStats(ctx context.Context, filter CarrierStatsFilter) (*model.CarrierStats, error)
	DataFreshness(ctx context.Context) (model.DataFreshness, error)
	Ping(ctx context.Context) error
	MigrationVersion(ctx context.Context) (MigrationVersion, error)
	CreateAPIKey(ctx context.Context, key *model.APIKey) error
	LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*model.APIKey, error)
	GetAPIKey(ctx context.Context, id string) (*model.APIKey, error)
	ListAPIKeys(ctx context.Context) ([]*model.APIKey, error)
	RevokeAPIKey(ctx context.Context, id string, revokedAt time.Time) error
	CountAPIKeys(ctx context.Context) (int64, error)
	ConsumeRateLimit(ctx context.Context, bucketKey string, requestsPerMinute int) (RateLimitResult, error)
	Close() error
}
