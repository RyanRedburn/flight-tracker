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
	CreateJobFn                           func(ctx context.Context, job *model.Job) error
	CreateFlightPerformanceIngestJobFn    func(ctx context.Context, year, month int) (*model.Job, error)
	CreateWeatherIngestJobFn              func(ctx context.Context, year, month int, stations []string) (*model.Job, error)
	GetJobFn                              func(ctx context.Context, id string) (*model.Job, error)
	GetFlightPerformanceIngestJobFn       func(ctx context.Context, jobID string) (*model.FlightPerformanceIngestJob, error)
	GetWeatherIngestJobFn                 func(ctx context.Context, jobID string) (*model.WeatherIngestJob, error)
	ListJobsFn                            func(ctx context.Context, limit int) ([]*model.Job, error)
	UpdateJobFn                           func(ctx context.Context, job *model.Job) error
	ClaimNextPendingJobFn                 func(ctx context.Context) (*model.Job, error)
	CompleteJobFn                         func(ctx context.Context, id string, result json.RawMessage) error
	FailJobFn                             func(ctx context.Context, id, errMsg string) error
	ResetStaleRunningJobsFn               func(ctx context.Context, olderThan time.Time) (int64, error)
	ActiveFlightPerformanceIngestMonthsFn func(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ActiveWeatherIngestMonthsFn           func(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ActiveIngestJobFn                     func(ctx context.Context, jobType string) (bool, error)
	CreateReferenceIngestJobFn            func(ctx context.Context, jobType string) (*model.Job, error)
	HasReferenceDataFn                    func(ctx context.Context, dataset store.ReferenceDataset) (bool, error)
	ReplaceCountriesFn                    func(ctx context.Context, columns []string, rows [][]string) error
	ReplaceRegionsFn                      func(ctx context.Context, columns []string, rows [][]string) error
	ReplaceAirportsFn                     func(ctx context.Context, columns []string, rows [][]string) error
	MonthsWithFlightPerformanceDataFn     func(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	MonthsWithWeatherDataFn               func(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	DistinctFlightAirportCodesFn          func(ctx context.Context) ([]string, error)
	ReplaceFlightPerformanceByMonthFn     func(ctx context.Context, year, month int, columns []string, rows [][]string) error
	ReplaceWeatherObservationsByMonthFn   func(ctx context.Context, year, month int, columns []string, rows [][]string) error
	RouteStatsFn                          func(ctx context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error)
	RouteOutlookFn                        func(ctx context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error)
	PingFn                                func(ctx context.Context) error
	MigrationVersionFn                    func(ctx context.Context) (store.MigrationVersion, error)
	CloseFn                               func() error
}

var _ store.Store = (*Stub)(nil)

func (s *Stub) CreateJob(ctx context.Context, job *model.Job) error {
	if s.CreateJobFn == nil {
		panic("unexpected call: CreateJob")
	}

	return s.CreateJobFn(ctx, job)
}

func (s *Stub) CreateFlightPerformanceIngestJob(ctx context.Context, year, month int) (*model.Job, error) {
	if s.CreateFlightPerformanceIngestJobFn == nil {
		panic("unexpected call: CreateFlightPerformanceIngestJob")
	}

	return s.CreateFlightPerformanceIngestJobFn(ctx, year, month)
}

func (s *Stub) CreateWeatherIngestJob(ctx context.Context, year, month int, stations []string) (*model.Job, error) {
	if s.CreateWeatherIngestJobFn == nil {
		panic("unexpected call: CreateWeatherIngestJob")
	}

	return s.CreateWeatherIngestJobFn(ctx, year, month, stations)
}

func (s *Stub) GetJob(ctx context.Context, id string) (*model.Job, error) {
	if s.GetJobFn == nil {
		panic("unexpected call: GetJob")
	}

	return s.GetJobFn(ctx, id)
}

func (s *Stub) GetFlightPerformanceIngestJob(ctx context.Context, jobID string) (*model.FlightPerformanceIngestJob, error) {
	if s.GetFlightPerformanceIngestJobFn == nil {
		panic("unexpected call: GetFlightPerformanceIngestJob")
	}

	return s.GetFlightPerformanceIngestJobFn(ctx, jobID)
}

func (s *Stub) GetWeatherIngestJob(ctx context.Context, jobID string) (*model.WeatherIngestJob, error) {
	if s.GetWeatherIngestJobFn == nil {
		panic("unexpected call: GetWeatherIngestJob")
	}

	return s.GetWeatherIngestJobFn(ctx, jobID)
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

func (s *Stub) ActiveFlightPerformanceIngestMonths(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	if s.ActiveFlightPerformanceIngestMonthsFn == nil {
		panic("unexpected call: ActiveFlightPerformanceIngestMonths")
	}

	return s.ActiveFlightPerformanceIngestMonthsFn(ctx, months)
}

func (s *Stub) ActiveWeatherIngestMonths(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	if s.ActiveWeatherIngestMonthsFn == nil {
		panic("unexpected call: ActiveWeatherIngestMonths")
	}

	return s.ActiveWeatherIngestMonthsFn(ctx, months)
}

func (s *Stub) ActiveIngestJob(ctx context.Context, jobType string) (bool, error) {
	if s.ActiveIngestJobFn == nil {
		panic("unexpected call: ActiveIngestJob")
	}

	return s.ActiveIngestJobFn(ctx, jobType)
}

func (s *Stub) CreateReferenceIngestJob(ctx context.Context, jobType string) (*model.Job, error) {
	if s.CreateReferenceIngestJobFn == nil {
		panic("unexpected call: CreateReferenceIngestJob")
	}

	return s.CreateReferenceIngestJobFn(ctx, jobType)
}

func (s *Stub) HasReferenceData(ctx context.Context, dataset store.ReferenceDataset) (bool, error) {
	if s.HasReferenceDataFn == nil {
		panic("unexpected call: HasReferenceData")
	}

	return s.HasReferenceDataFn(ctx, dataset)
}

func (s *Stub) ReplaceCountries(ctx context.Context, columns []string, rows [][]string) error {
	if s.ReplaceCountriesFn == nil {
		panic("unexpected call: ReplaceCountries")
	}

	return s.ReplaceCountriesFn(ctx, columns, rows)
}

func (s *Stub) ReplaceRegions(ctx context.Context, columns []string, rows [][]string) error {
	if s.ReplaceRegionsFn == nil {
		panic("unexpected call: ReplaceRegions")
	}

	return s.ReplaceRegionsFn(ctx, columns, rows)
}

func (s *Stub) ReplaceAirports(ctx context.Context, columns []string, rows [][]string) error {
	if s.ReplaceAirportsFn == nil {
		panic("unexpected call: ReplaceAirports")
	}

	return s.ReplaceAirportsFn(ctx, columns, rows)
}

func (s *Stub) MonthsWithFlightPerformanceData(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	if s.MonthsWithFlightPerformanceDataFn == nil {
		panic("unexpected call: MonthsWithFlightPerformanceData")
	}

	return s.MonthsWithFlightPerformanceDataFn(ctx, months)
}

func (s *Stub) MonthsWithWeatherData(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error) {
	if s.MonthsWithWeatherDataFn == nil {
		panic("unexpected call: MonthsWithWeatherData")
	}

	return s.MonthsWithWeatherDataFn(ctx, months)
}

func (s *Stub) DistinctFlightAirportCodes(ctx context.Context) ([]string, error) {
	if s.DistinctFlightAirportCodesFn == nil {
		panic("unexpected call: DistinctFlightAirportCodes")
	}

	return s.DistinctFlightAirportCodesFn(ctx)
}

func (s *Stub) ReplaceFlightPerformanceByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error {
	if s.ReplaceFlightPerformanceByMonthFn == nil {
		panic("unexpected call: ReplaceFlightPerformanceByMonth")
	}

	return s.ReplaceFlightPerformanceByMonthFn(ctx, year, month, columns, rows)
}

func (s *Stub) ReplaceWeatherObservationsByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error {
	if s.ReplaceWeatherObservationsByMonthFn == nil {
		panic("unexpected call: ReplaceWeatherObservationsByMonth")
	}

	return s.ReplaceWeatherObservationsByMonthFn(ctx, year, month, columns, rows)
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
