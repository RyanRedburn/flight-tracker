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
	CreateFlightPerformanceIngestJobFn    func(ctx context.Context, year, month int) (*model.Job, error)
	CreateWeatherIngestJobFn              func(ctx context.Context, year, month int, stations []string) (*model.Job, error)
	GetJobFn                              func(ctx context.Context, id string) (*model.Job, error)
	GetFlightPerformanceIngestJobFn       func(ctx context.Context, jobID string) (*model.FlightPerformanceIngestJob, error)
	GetWeatherIngestJobFn                 func(ctx context.Context, jobID string) (*model.WeatherIngestJob, error)
	ListJobsFn                            func(ctx context.Context, limit int) ([]*model.Job, error)
	ClaimNextPendingJobFn                 func(ctx context.Context, leaseUntil time.Time) (*model.Job, error)
	CompleteJobFn                         func(ctx context.Context, id string, result json.RawMessage) error
	FailJobFn                             func(ctx context.Context, id, errMsg string) error
	HeartbeatJobFn                        func(ctx context.Context, id string, leaseUntil time.Time) error
	ResetStaleRunningJobsFn               func(ctx context.Context, expiredBefore time.Time) (int64, error)
	ActiveFlightPerformanceIngestMonthsFn func(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ActiveWeatherIngestMonthsFn           func(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	ActiveIngestJobFn                     func(ctx context.Context, jobType model.JobType) (bool, error)
	CreateReferenceIngestJobFn            func(ctx context.Context, jobType model.JobType) (*model.Job, error)
	CreateRebuildRouteTravelWindowsJobFn  func(ctx context.Context) (*model.Job, error)
	CreateRebuildRouteWeatherStatsJobFn   func(ctx context.Context) (*model.Job, error)
	HasReferenceDataFn                    func(ctx context.Context, dataset store.ReferenceDataset) (bool, error)
	ReplaceCountriesFn                    func(ctx context.Context, columns []string, rows [][]string) error
	ReplaceRegionsFn                      func(ctx context.Context, columns []string, rows [][]string) error
	ReplaceAirportsFn                     func(ctx context.Context, columns []string, rows [][]string) error
	HasWeatherStationsDataFn              func(ctx context.Context) (bool, error)
	ReplaceWeatherStationsFn              func(ctx context.Context, stationColumns []string, stationRows [][]string, mappingColumns []string, mappingRows [][]string) error
	ListAirportWeatherStationsFn          func(ctx context.Context) ([]store.AirportWeatherStation, error)
	ListAirportIdentifiersByIATAFn        func(ctx context.Context, codes []string) (map[string]store.AirportIdentifiers, error)
	MonthsWithFlightPerformanceDataFn     func(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	MonthsWithWeatherDataFn               func(ctx context.Context, months []model.YearMonth) ([]model.YearMonth, error)
	DistinctFlightAirportCodesFn          func(ctx context.Context) ([]string, error)
	ReplaceFlightPerformanceByMonthFn     func(ctx context.Context, year, month int, columns []string, rows [][]string) error
	ReplaceWeatherObservationsByMonthFn   func(ctx context.Context, year, month int, columns []string, rows [][]string) error
	RouteStatsFn                          func(ctx context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error)
	RouteOutlookFn                        func(ctx context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error)
	RouteTravelWindowsFn                  func(ctx context.Context, filter store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error)
	RebuildRouteTravelWindowsFn           func(ctx context.Context) error
	RouteWeatherStatsFn                   func(ctx context.Context, filter store.RouteStatsFilter) (*model.RouteWeatherStats, error)
	RebuildRouteWeatherStatsFn            func(ctx context.Context) error
	CarrierStatsFn                        func(ctx context.Context, filter store.CarrierStatsFilter) (*model.CarrierStats, error)
	DataFreshnessFn                       func(ctx context.Context) (model.DataFreshness, error)
	PingFn                                func(ctx context.Context) error
	MigrationVersionFn                    func(ctx context.Context) (store.MigrationVersion, error)
	CreateAPIKeyFn                        func(ctx context.Context, key *model.APIKey) error
	LookupAPIKeyByPrefixFn                func(ctx context.Context, prefix string) (*model.APIKey, error)
	GetAPIKeyFn                           func(ctx context.Context, id string) (*model.APIKey, error)
	ListAPIKeysFn                         func(ctx context.Context) ([]*model.APIKey, error)
	RevokeAPIKeyFn                        func(ctx context.Context, id string, revokedAt time.Time) error
	CountAPIKeysFn                        func(ctx context.Context) (int64, error)
	ConsumeRateLimitFn                    func(ctx context.Context, bucketKey string, requestsPerMinute int) (store.RateLimitResult, error)
	DeleteStaleRateLimitBucketsFn         func(ctx context.Context) error
	CloseFn                               func() error
}

var _ store.Store = (*Stub)(nil)

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

func (s *Stub) ClaimNextPendingJob(ctx context.Context, leaseUntil time.Time) (*model.Job, error) {
	if s.ClaimNextPendingJobFn == nil {
		panic("unexpected call: ClaimNextPendingJob")
	}

	return s.ClaimNextPendingJobFn(ctx, leaseUntil)
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

func (s *Stub) HeartbeatJob(ctx context.Context, id string, leaseUntil time.Time) error {
	if s.HeartbeatJobFn == nil {
		panic("unexpected call: HeartbeatJob")
	}

	return s.HeartbeatJobFn(ctx, id, leaseUntil)
}

func (s *Stub) ResetStaleRunningJobs(ctx context.Context, expiredBefore time.Time) (int64, error) {
	if s.ResetStaleRunningJobsFn == nil {
		panic("unexpected call: ResetStaleRunningJobs")
	}

	return s.ResetStaleRunningJobsFn(ctx, expiredBefore)
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

func (s *Stub) ActiveIngestJob(ctx context.Context, jobType model.JobType) (bool, error) {
	if s.ActiveIngestJobFn == nil {
		panic("unexpected call: ActiveIngestJob")
	}

	return s.ActiveIngestJobFn(ctx, jobType)
}

func (s *Stub) CreateReferenceIngestJob(ctx context.Context, jobType model.JobType) (*model.Job, error) {
	if s.CreateReferenceIngestJobFn == nil {
		panic("unexpected call: CreateReferenceIngestJob")
	}

	return s.CreateReferenceIngestJobFn(ctx, jobType)
}

func (s *Stub) CreateRebuildRouteTravelWindowsJob(ctx context.Context) (*model.Job, error) {
	if s.CreateRebuildRouteTravelWindowsJobFn == nil {
		panic("unexpected call: CreateRebuildRouteTravelWindowsJob")
	}

	return s.CreateRebuildRouteTravelWindowsJobFn(ctx)
}

func (s *Stub) CreateRebuildRouteWeatherStatsJob(ctx context.Context) (*model.Job, error) {
	if s.CreateRebuildRouteWeatherStatsJobFn == nil {
		panic("unexpected call: CreateRebuildRouteWeatherStatsJob")
	}

	return s.CreateRebuildRouteWeatherStatsJobFn(ctx)
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

func (s *Stub) HasWeatherStationsData(ctx context.Context) (bool, error) {
	if s.HasWeatherStationsDataFn == nil {
		panic("unexpected call: HasWeatherStationsData")
	}

	return s.HasWeatherStationsDataFn(ctx)
}

func (s *Stub) ReplaceWeatherStations(
	ctx context.Context,
	stationColumns []string,
	stationRows [][]string,
	mappingColumns []string,
	mappingRows [][]string,
) error {
	if s.ReplaceWeatherStationsFn == nil {
		panic("unexpected call: ReplaceWeatherStations")
	}

	return s.ReplaceWeatherStationsFn(ctx, stationColumns, stationRows, mappingColumns, mappingRows)
}

func (s *Stub) ListAirportWeatherStations(ctx context.Context) ([]store.AirportWeatherStation, error) {
	if s.ListAirportWeatherStationsFn == nil {
		panic("unexpected call: ListAirportWeatherStations")
	}

	return s.ListAirportWeatherStationsFn(ctx)
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

func (s *Stub) ListAirportIdentifiersByIATA(ctx context.Context, codes []string) (map[string]store.AirportIdentifiers, error) {
	if s.ListAirportIdentifiersByIATAFn == nil {
		panic("unexpected call: ListAirportIdentifiersByIATA")
	}

	return s.ListAirportIdentifiersByIATAFn(ctx, codes)
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

func (s *Stub) RouteTravelWindows(ctx context.Context, filter store.RouteTravelWindowsFilter) (*model.RouteTravelWindows, error) {
	if s.RouteTravelWindowsFn == nil {
		panic("unexpected call: RouteTravelWindows")
	}

	return s.RouteTravelWindowsFn(ctx, filter)
}

func (s *Stub) RebuildRouteTravelWindows(ctx context.Context) error {
	if s.RebuildRouteTravelWindowsFn == nil {
		panic("unexpected call: RebuildRouteTravelWindows")
	}

	return s.RebuildRouteTravelWindowsFn(ctx)
}

func (s *Stub) RouteWeatherStats(ctx context.Context, filter store.RouteStatsFilter) (*model.RouteWeatherStats, error) {
	if s.RouteWeatherStatsFn == nil {
		panic("unexpected call: RouteWeatherStats")
	}

	return s.RouteWeatherStatsFn(ctx, filter)
}

func (s *Stub) RebuildRouteWeatherStats(ctx context.Context) error {
	if s.RebuildRouteWeatherStatsFn == nil {
		panic("unexpected call: RebuildRouteWeatherStats")
	}

	return s.RebuildRouteWeatherStatsFn(ctx)
}

func (s *Stub) CarrierStats(ctx context.Context, filter store.CarrierStatsFilter) (*model.CarrierStats, error) {
	if s.CarrierStatsFn == nil {
		panic("unexpected call: CarrierStats")
	}

	return s.CarrierStatsFn(ctx, filter)
}

func (s *Stub) DataFreshness(ctx context.Context) (model.DataFreshness, error) {
	if s.DataFreshnessFn == nil {
		panic("unexpected call: DataFreshness")
	}

	return s.DataFreshnessFn(ctx)
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

func (s *Stub) CreateAPIKey(ctx context.Context, key *model.APIKey) error {
	if s.CreateAPIKeyFn == nil {
		panic("unexpected call: CreateAPIKey")
	}

	return s.CreateAPIKeyFn(ctx, key)
}

func (s *Stub) LookupAPIKeyByPrefix(ctx context.Context, prefix string) (*model.APIKey, error) {
	if s.LookupAPIKeyByPrefixFn == nil {
		panic("unexpected call: LookupAPIKeyByPrefix")
	}

	return s.LookupAPIKeyByPrefixFn(ctx, prefix)
}

func (s *Stub) GetAPIKey(ctx context.Context, id string) (*model.APIKey, error) {
	if s.GetAPIKeyFn == nil {
		panic("unexpected call: GetAPIKey")
	}

	return s.GetAPIKeyFn(ctx, id)
}

func (s *Stub) ListAPIKeys(ctx context.Context) ([]*model.APIKey, error) {
	if s.ListAPIKeysFn == nil {
		panic("unexpected call: ListAPIKeys")
	}

	return s.ListAPIKeysFn(ctx)
}

func (s *Stub) RevokeAPIKey(ctx context.Context, id string, revokedAt time.Time) error {
	if s.RevokeAPIKeyFn == nil {
		panic("unexpected call: RevokeAPIKey")
	}

	return s.RevokeAPIKeyFn(ctx, id, revokedAt)
}

func (s *Stub) CountAPIKeys(ctx context.Context) (int64, error) {
	if s.CountAPIKeysFn == nil {
		panic("unexpected call: CountAPIKeys")
	}

	return s.CountAPIKeysFn(ctx)
}

func (s *Stub) ConsumeRateLimit(ctx context.Context, bucketKey string, requestsPerMinute int) (store.RateLimitResult, error) {
	if s.ConsumeRateLimitFn == nil {
		panic("unexpected call: ConsumeRateLimit")
	}

	return s.ConsumeRateLimitFn(ctx, bucketKey, requestsPerMinute)
}

func (s *Stub) DeleteStaleRateLimitBuckets(ctx context.Context) error {
	if s.DeleteStaleRateLimitBucketsFn == nil {
		panic("unexpected call: DeleteStaleRateLimitBuckets")
	}

	return s.DeleteStaleRateLimitBucketsFn(ctx)
}

func (s *Stub) Close() error {
	if s.CloseFn == nil {
		panic("unexpected call: Close")
	}

	return s.CloseFn()
}
