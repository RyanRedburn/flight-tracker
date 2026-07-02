package store

import (
	"context"
	"errors"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

var ErrNotFound = errors.New("not found")

type MigrationVersion struct {
	Version uint `json:"version"`
	Dirty   bool `json:"dirty"`
}

type OnTimeFlightFilter struct {
	FlightDate string `validate:"omitempty,datetime=2006-01-02"`
	Origin     string `validate:"omitempty,len=3"`
	Dest       string `validate:"omitempty,len=3"`
	Limit      int
	Offset     int
}

type Store interface {
	CreateJob(ctx context.Context, job *model.Job) error
	GetJob(ctx context.Context, id string) (*model.Job, error)
	ListJobs(ctx context.Context, limit int) ([]*model.Job, error)
	UpdateJob(ctx context.Context, job *model.Job) error
	ListOnTimeFlights(ctx context.Context, filter OnTimeFlightFilter) ([]*model.OnTimeFlight, error)
	Ping(ctx context.Context) error
	MigrationVersion(ctx context.Context) (MigrationVersion, error)
	Close() error
}
