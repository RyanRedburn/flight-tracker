package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrJobStatusConflict = errors.New("job status conflict")
)

type MigrationVersion struct {
	Version uint `json:"version"`
	Dirty   bool `json:"dirty"`
}

type Store interface {
	CreateJob(ctx context.Context, job *model.Job) error
	CreateFlightPerformanceIngestJob(ctx context.Context, year, month int) (*model.Job, error)
	GetJob(ctx context.Context, id string) (*model.Job, error)
	GetFlightPerformanceIngestJob(ctx context.Context, jobID string) (*model.FlightPerformanceIngestJob, error)
	ListJobs(ctx context.Context, limit int) ([]*model.Job, error)
	UpdateJob(ctx context.Context, job *model.Job) error
	ClaimNextPendingJob(ctx context.Context) (*model.Job, error)
	CompleteJob(ctx context.Context, id string, result json.RawMessage) error
	FailJob(ctx context.Context, id, errMsg string) error
	ResetStaleRunningJobs(ctx context.Context, olderThan time.Time) (int64, error)
	ActiveFlightPerformanceIngestMonths(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ActiveIngestJob(ctx context.Context, jobType string) (bool, error)
	CreateReferenceIngestJob(ctx context.Context, jobType string) (*model.Job, error)
	HasReferenceData(ctx context.Context, dataset ReferenceDataset) (bool, error)
	ReplaceCountries(ctx context.Context, columns []string, rows [][]string) error
	ReplaceRegions(ctx context.Context, columns []string, rows [][]string) error
	ReplaceAirports(ctx context.Context, columns []string, rows [][]string) error
	MonthsWithFlightPerformanceData(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ReplaceFlightPerformanceByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error
	RouteStats(ctx context.Context, filter RouteStatsFilter) (*model.RouteStats, error)
	RouteOutlook(ctx context.Context, filter RouteOutlookFilter) (*model.RouteOutlook, error)
	Ping(ctx context.Context) error
	MigrationVersion(ctx context.Context) (MigrationVersion, error)
	Close() error
}
