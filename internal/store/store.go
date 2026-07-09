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
	CreateBTSIngestJob(ctx context.Context, year, month int) (*model.Job, error)
	GetJob(ctx context.Context, id string) (*model.Job, error)
	GetBTSIngestJob(ctx context.Context, jobID string) (*model.BTSIngestJob, error)
	ListJobs(ctx context.Context, limit int) ([]*model.Job, error)
	UpdateJob(ctx context.Context, job *model.Job) error
	ClaimNextPendingJob(ctx context.Context) (*model.Job, error)
	CompleteJob(ctx context.Context, id string, result json.RawMessage) error
	FailJob(ctx context.Context, id, errMsg string) error
	ResetStaleRunningJobs(ctx context.Context, olderThan time.Time) (int64, error)
	ActiveBTSIngestMonths(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ActiveIngestJob(ctx context.Context, jobType string) (bool, error)
	MonthsWithOnTimeFlightData(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ReplaceOnTimeFlightsByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error
	RouteStats(ctx context.Context, filter RouteStatsFilter) (*model.RouteStats, error)
	RouteOutlook(ctx context.Context, filter RouteOutlookFilter) (*model.RouteOutlook, error)
	Ping(ctx context.Context) error
	MigrationVersion(ctx context.Context) (MigrationVersion, error)
	Close() error
}
