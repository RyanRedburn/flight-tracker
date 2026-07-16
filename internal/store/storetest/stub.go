package storetest

import (
	"context"
	"encoding/json"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

// Stub is a scenario-based store.Store for unit tests.
// Set Fn fields to hard-coded returns for the scenario under test.
// Unset Fns panic so missing setup fails loudly.
type Stub struct {
	CreateJobFn                   func(ctx context.Context, job *model.Job) error
	CreateBTSIngestJobFn          func(ctx context.Context, year, month int) (*model.Job, error)
	GetJobFn                      func(ctx context.Context, id string) (*model.Job, error)
	GetBTSIngestJobFn             func(ctx context.Context, jobID string) (*model.BTSIngestJob, error)
	ListJobsFn                    func(ctx context.Context, limit int) ([]*model.Job, error)
	UpdateJobFn                   func(ctx context.Context, job *model.Job) error
	ClaimNextPendingJobFn         func(ctx context.Context) (*model.Job, error)
	CompleteJobFn                 func(ctx context.Context, id string, result json.RawMessage) error
	FailJobFn                     func(ctx context.Context, id, errMsg string) error
	ResetStaleRunningJobsFn       func(ctx context.Context, olderThan time.Time) (int64, error)
	ActiveBTSIngestMonthsFn       func(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ActiveIngestJobFn             func(ctx context.Context, jobType string) (bool, error)
	CreateOurAirportsIngestJobFn  func(ctx context.Context, jobType string) (*model.Job, error)
	HasOurAirportsDataFn          func(ctx context.Context, dataset store.OurAirportsDataset) (bool, error)
	ReplaceOurAirportsCountriesFn func(ctx context.Context, columns []string, rows [][]string) error
	ReplaceOurAirportsRegionsFn   func(ctx context.Context, columns []string, rows [][]string) error
	ReplaceOurAirportsAirportsFn  func(ctx context.Context, columns []string, rows [][]string) error
	MonthsWithOnTimeFlightDataFn  func(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ReplaceOnTimeFlightsByMonthFn func(ctx context.Context, year, month int, columns []string, rows [][]string) error
	RouteStatsFn                  func(ctx context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error)
	RouteOutlookFn                func(ctx context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error)
	PingFn                        func(ctx context.Context) error
	MigrationVersionFn            func(ctx context.Context) (store.MigrationVersion, error)
	CloseFn                       func() error
}

var _ store.Store = (*Stub)(nil)

func (s *Stub) CreateJob(ctx context.Context, job *model.Job) error {
	if s.CreateJobFn == nil {
		panic("unexpected call: CreateJob")
	}

	return s.CreateJobFn(ctx, job)
}

func (s *Stub) CreateBTSIngestJob(ctx context.Context, year, month int) (*model.Job, error) {
	if s.CreateBTSIngestJobFn == nil {
		panic("unexpected call: CreateBTSIngestJob")
	}

	return s.CreateBTSIngestJobFn(ctx, year, month)
}

func (s *Stub) GetJob(ctx context.Context, id string) (*model.Job, error) {
	if s.GetJobFn == nil {
		panic("unexpected call: GetJob")
	}

	return s.GetJobFn(ctx, id)
}

func (s *Stub) GetBTSIngestJob(ctx context.Context, jobID string) (*model.BTSIngestJob, error) {
	if s.GetBTSIngestJobFn == nil {
		panic("unexpected call: GetBTSIngestJob")
	}

	return s.GetBTSIngestJobFn(ctx, jobID)
}

func (s *Stub) ListJobs(ctx context.Context, limit int) ([]*model.Job, error) {
	if s.ListJobsFn == nil {
		panic("unexpected call: ListJobs")
	}

	return s.ListJobsFn(ctx, limit)
}

func (s *Stub) UpdateJob(ctx context.Context, job *model.Job) error {
	if s.UpdateJobFn == nil {
		panic("unexpected call: UpdateJob")
	}

	return s.UpdateJobFn(ctx, job)
}

func (s *Stub) ClaimNextPendingJob(ctx context.Context) (*model.Job, error) {
	if s.ClaimNextPendingJobFn == nil {
		panic("unexpected call: ClaimNextPendingJob")
	}

	return s.ClaimNextPendingJobFn(ctx)
}

func (s *Stub) CompleteJob(ctx context.Context, id string, result json.RawMessage) error {
	if s.CompleteJobFn == nil {
		panic("unexpected call: CompleteJob")
	}

	return s.CompleteJobFn(ctx, id, result)
}

func (s *Stub) FailJob(ctx context.Context, id, errMsg string) error {
	if s.FailJobFn == nil {
		panic("unexpected call: FailJob")
	}

	return s.FailJobFn(ctx, id, errMsg)
}

func (s *Stub) ResetStaleRunningJobs(ctx context.Context, olderThan time.Time) (int64, error) {
	if s.ResetStaleRunningJobsFn == nil {
		panic("unexpected call: ResetStaleRunningJobs")
	}

	return s.ResetStaleRunningJobsFn(ctx, olderThan)
}

func (s *Stub) ActiveBTSIngestMonths(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	if s.ActiveBTSIngestMonthsFn == nil {
		panic("unexpected call: ActiveBTSIngestMonths")
	}

	return s.ActiveBTSIngestMonthsFn(ctx, months)
}

func (s *Stub) ActiveIngestJob(ctx context.Context, jobType string) (bool, error) {
	if s.ActiveIngestJobFn == nil {
		panic("unexpected call: ActiveIngestJob")
	}

	return s.ActiveIngestJobFn(ctx, jobType)
}

func (s *Stub) CreateOurAirportsIngestJob(ctx context.Context, jobType string) (*model.Job, error) {
	if s.CreateOurAirportsIngestJobFn == nil {
		panic("unexpected call: CreateOurAirportsIngestJob")
	}

	return s.CreateOurAirportsIngestJobFn(ctx, jobType)
}

func (s *Stub) HasOurAirportsData(ctx context.Context, dataset store.OurAirportsDataset) (bool, error) {
	if s.HasOurAirportsDataFn == nil {
		panic("unexpected call: HasOurAirportsData")
	}

	return s.HasOurAirportsDataFn(ctx, dataset)
}

func (s *Stub) ReplaceOurAirportsCountries(ctx context.Context, columns []string, rows [][]string) error {
	if s.ReplaceOurAirportsCountriesFn == nil {
		panic("unexpected call: ReplaceOurAirportsCountries")
	}

	return s.ReplaceOurAirportsCountriesFn(ctx, columns, rows)
}

func (s *Stub) ReplaceOurAirportsRegions(ctx context.Context, columns []string, rows [][]string) error {
	if s.ReplaceOurAirportsRegionsFn == nil {
		panic("unexpected call: ReplaceOurAirportsRegions")
	}

	return s.ReplaceOurAirportsRegionsFn(ctx, columns, rows)
}

func (s *Stub) ReplaceOurAirportsAirports(ctx context.Context, columns []string, rows [][]string) error {
	if s.ReplaceOurAirportsAirportsFn == nil {
		panic("unexpected call: ReplaceOurAirportsAirports")
	}

	return s.ReplaceOurAirportsAirportsFn(ctx, columns, rows)
}

func (s *Stub) MonthsWithOnTimeFlightData(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	if s.MonthsWithOnTimeFlightDataFn == nil {
		panic("unexpected call: MonthsWithOnTimeFlightData")
	}

	return s.MonthsWithOnTimeFlightDataFn(ctx, months)
}

func (s *Stub) ReplaceOnTimeFlightsByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error {
	if s.ReplaceOnTimeFlightsByMonthFn == nil {
		panic("unexpected call: ReplaceOnTimeFlightsByMonth")
	}

	return s.ReplaceOnTimeFlightsByMonthFn(ctx, year, month, columns, rows)
}

func (s *Stub) RouteStats(ctx context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error) {
	if s.RouteStatsFn == nil {
		panic("unexpected call: RouteStats")
	}

	return s.RouteStatsFn(ctx, filter)
}

func (s *Stub) RouteOutlook(ctx context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
	if s.RouteOutlookFn == nil {
		panic("unexpected call: RouteOutlook")
	}

	return s.RouteOutlookFn(ctx, filter)
}

func (s *Stub) Ping(ctx context.Context) error {
	if s.PingFn == nil {
		panic("unexpected call: Ping")
	}

	return s.PingFn(ctx)
}

func (s *Stub) MigrationVersion(ctx context.Context) (store.MigrationVersion, error) {
	if s.MigrationVersionFn == nil {
		panic("unexpected call: MigrationVersion")
	}

	return s.MigrationVersionFn(ctx)
}

func (s *Stub) Close() error {
	if s.CloseFn == nil {
		panic("unexpected call: Close")
	}

	return s.CloseFn()
}
