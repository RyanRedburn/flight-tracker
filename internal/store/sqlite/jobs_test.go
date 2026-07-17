//go:build cgo

package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const (
	testFlightDate20260424 = "2026-04-24"
	testFlightDate20260425 = "2026-04-25"
	testFlightDate20260430 = "2026-04-30"
	testAirportORD         = "ORD"
	testAirportBHM         = "BHM"
	testAirportAVP         = "AVP"
	testAirportLAX         = "LAX"
	testAirportSFO         = "SFO"
	testFloatNo            = "0.00"
	testFloatYes           = "1.00"
)

func openTestStore(t *testing.T) store.Store {
	t.Helper()

	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	migrationsPath := filepath.Join("..", "..", "..", "migrations", "sqlite")

	s, err := Open(ctx, "file:"+filepath.ToSlash(dbPath), migrationsPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	t.Cleanup(func() { _ = s.Close() })

	return s
}

func TestCreateFlightPerformanceIngestJob(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	job, err := s.CreateFlightPerformanceIngestJob(ctx, 2026, 4)
	if err != nil {
		t.Fatalf("CreateFlightPerformanceIngestJob() error = %v", err)
	}

	if job.Type != model.JobTypeImportFlightPerformance {
		t.Errorf("Type = %q, want %q", job.Type, model.JobTypeImportFlightPerformance)
	}

	if job.Status != model.JobStatusPending {
		t.Errorf("Status = %q, want pending", job.Status)
	}

	detail, err := s.GetFlightPerformanceIngestJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetFlightPerformanceIngestJob() error = %v", err)
	}

	if detail.Year != 2026 || detail.Month != 4 {
		t.Errorf("detail = %+v, want year 2026 month 4", detail)
	}
}

func TestClaimNextPendingJob(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	if _, err := s.ClaimNextPendingJob(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("ClaimNextPendingJob() error = %v, want ErrNotFound", err)
	}

	created, err := s.CreateFlightPerformanceIngestJob(ctx, 2026, 1)
	if err != nil {
		t.Fatalf("CreateFlightPerformanceIngestJob() error = %v", err)
	}

	claimed, err := s.ClaimNextPendingJob(ctx)
	if err != nil {
		t.Fatalf("ClaimNextPendingJob() error = %v", err)
	}

	if claimed.ID != created.ID {
		t.Errorf("claimed ID = %q, want %q", claimed.ID, created.ID)
	}

	if claimed.Status != model.JobStatusRunning {
		t.Errorf("Status = %q, want running", claimed.Status)
	}

	if claimed.StartedAt == nil {
		t.Fatal("StartedAt is nil, want timestamp")
	}

	if _, err := s.ClaimNextPendingJob(ctx); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second ClaimNextPendingJob() error = %v, want ErrNotFound", err)
	}
}

func TestClaimNextPendingJobConcurrent(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	for range 3 {
		if _, err := s.CreateFlightPerformanceIngestJob(ctx, 2026, 1); err != nil {
			t.Fatalf("CreateFlightPerformanceIngestJob() error = %v", err)
		}
	}

	var wg sync.WaitGroup

	const workers = 8

	claimed := make(chan string, workers)

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()

			job, err := s.ClaimNextPendingJob(ctx)
			if errors.Is(err, store.ErrNotFound) {
				return
			}

			if err != nil {
				t.Errorf("ClaimNextPendingJob() error = %v", err)
				return
			}

			claimed <- job.ID
		}()
	}

	wg.Wait()
	close(claimed)

	ids := make(map[string]struct{})
	for id := range claimed {
		if _, dup := ids[id]; dup {
			t.Errorf("job %q claimed more than once", id)
		}

		ids[id] = struct{}{}
	}

	if len(ids) != 3 {
		t.Errorf("claimed %d jobs, want 3", len(ids))
	}
}

func TestCompleteAndFailJob(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	job, err := s.CreateFlightPerformanceIngestJob(ctx, 2026, 2)
	if err != nil {
		t.Fatalf("CreateFlightPerformanceIngestJob() error = %v", err)
	}

	claimed, err := s.ClaimNextPendingJob(ctx)
	if err != nil {
		t.Fatalf("ClaimNextPendingJob() error = %v", err)
	}

	result := []byte(`{"rows_imported":10}`)
	if err := s.CompleteJob(ctx, claimed.ID, result); err != nil {
		t.Fatalf("CompleteJob() error = %v", err)
	}

	if err := s.CompleteJob(ctx, claimed.ID, result); !errors.Is(err, store.ErrJobStatusConflict) {
		t.Fatalf("second CompleteJob() error = %v, want ErrJobStatusConflict", err)
	}

	completed, err := s.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if completed.Status != model.JobStatusCompleted {
		t.Errorf("Status = %q, want completed", completed.Status)
	}

	if completed.EndedAt == nil {
		t.Fatal("EndedAt is nil, want timestamp")
	}

	failJob, err := s.CreateFlightPerformanceIngestJob(ctx, 2026, 3)
	if err != nil {
		t.Fatalf("CreateFlightPerformanceIngestJob() error = %v", err)
	}

	if _, err := s.ClaimNextPendingJob(ctx); err != nil {
		t.Fatalf("ClaimNextPendingJob() error = %v", err)
	}

	if err := s.FailJob(ctx, failJob.ID, "download failed"); err != nil {
		t.Fatalf("FailJob() error = %v", err)
	}

	failed, err := s.GetJob(ctx, failJob.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if failed.Status != model.JobStatusFailed {
		t.Errorf("Status = %q, want failed", failed.Status)
	}

	if failed.Error != "download failed" {
		t.Errorf("Error = %q, want download failed", failed.Error)
	}
}

func TestActiveFlightPerformanceIngestMonths(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	months := []model.YearMonth{{Year: 2026, Month: 4}, {Year: 2026, Month: 5}}

	active, err := s.ActiveFlightPerformanceIngestMonths(ctx, months)
	if err != nil {
		t.Fatalf("ActiveFlightPerformanceIngestMonths() error = %v", err)
	}

	if len(active) != 0 {
		t.Fatalf("len(active) = %d, want 0", len(active))
	}

	if _, err := s.CreateFlightPerformanceIngestJob(ctx, 2026, 4); err != nil {
		t.Fatalf("CreateFlightPerformanceIngestJob() error = %v", err)
	}

	active, err = s.ActiveFlightPerformanceIngestMonths(ctx, months)
	if err != nil {
		t.Fatalf("ActiveFlightPerformanceIngestMonths() error = %v", err)
	}

	if len(active) != 1 {
		t.Fatalf("len(active) = %d, want 1", len(active))
	}

	if active[0].Year != 2026 || active[0].Month != 4 {
		t.Errorf("active[0] = %+v, want 2026-04", active[0])
	}
}

func TestActiveIngestJob(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	active, err := s.ActiveIngestJob(ctx, model.JobTypeImportFlightPerformance)
	if err != nil {
		t.Fatalf("ActiveIngestJob() error = %v", err)
	}

	if active {
		t.Fatal("expected no active job before create")
	}

	if _, err := s.CreateFlightPerformanceIngestJob(ctx, 2026, 4); err != nil {
		t.Fatalf("CreateFlightPerformanceIngestJob() error = %v", err)
	}

	active, err = s.ActiveIngestJob(ctx, model.JobTypeImportFlightPerformance)
	if err != nil {
		t.Fatalf("ActiveIngestJob() error = %v", err)
	}

	if !active {
		t.Fatal("expected active job after create")
	}
}

func TestActiveIngestJobReference(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	if _, err := s.CreateReferenceIngestJob(ctx, model.JobTypeImportAirports); err != nil {
		t.Fatalf("CreateReferenceIngestJob() error = %v", err)
	}

	active, err := s.ActiveIngestJob(ctx, model.JobTypeImportAirports)
	if err != nil {
		t.Fatalf("ActiveIngestJob() error = %v", err)
	}

	if !active {
		t.Fatal("expected active reference airports job")
	}

	active, err = s.ActiveIngestJob(ctx, model.JobTypeImportCountries)
	if err != nil {
		t.Fatalf("ActiveIngestJob() error = %v", err)
	}

	if active {
		t.Fatal("expected no active countries job when only airports is pending")
	}
}

func testFlightColumns() []string {
	return []string{
		"year", "month", "flight_date", "origin", "dest",
		"iata_code_marketing_airline", "flight_number_marketing_airline", "crs_dep_time",
	}
}

func testFlightRow(flightDate, origin, dest, airline, flightNum, crsDep string) []string {
	year, month, ok := store.FlightYearMonthFromDate(flightDate)
	if !ok {
		panic("invalid flight date: " + flightDate)
	}

	return []string{
		strconv.Itoa(year),
		strconv.Itoa(month),
		flightDate, origin, dest, airline, flightNum, crsDep,
	}
}

func TestMonthsWithFlightPerformanceData(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	columns := testFlightColumns()

	rows := [][]string{testFlightRow(testFlightDate20260424, testAirportORD, testAirportBHM, "UA", "4547", "1535")}
	if err := s.ReplaceFlightPerformanceByMonth(ctx, 2026, 4, columns, rows); err != nil {
		t.Fatalf("ReplaceFlightPerformanceByMonth() error = %v", err)
	}

	months := []model.YearMonth{{Year: 2026, Month: 4}, {Year: 2026, Month: 5}}

	withData, err := s.MonthsWithFlightPerformanceData(ctx, months)
	if err != nil {
		t.Fatalf("MonthsWithFlightPerformanceData() error = %v", err)
	}

	if len(withData) != 1 {
		t.Fatalf("len(withData) = %d, want 1", len(withData))
	}

	if withData[0].Month != 4 {
		t.Errorf("withData[0] = %+v, want month 4", withData[0])
	}
}

func TestReplaceFlightPerformanceByMonthRollback(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	columns := testFlightColumns()

	seed := [][]string{testFlightRow(testFlightDate20260424, testAirportORD, testAirportBHM, "UA", "4547", "1535")}
	if err := s.ReplaceFlightPerformanceByMonth(ctx, 2026, 4, columns, seed); err != nil {
		t.Fatalf("seed ReplaceFlightPerformanceByMonth() error = %v", err)
	}

	badColumns := []string{"flight_date", "not_a_column"}

	badRows := [][]string{{testFlightDate20260425, testAirportLAX}}
	if err := s.ReplaceFlightPerformanceByMonth(ctx, 2026, 4, badColumns, badRows); err == nil {
		t.Fatal("ReplaceFlightPerformanceByMonth() expected error for invalid column")
	}

	stats, err := s.RouteStats(ctx, store.RouteStatsFilter{
		Origin:    testAirportORD,
		Dest:      testAirportBHM,
		StartDate: testFlightDate20260424,
		EndDate:   testFlightDate20260424,
	})
	if err != nil {
		t.Fatalf("RouteStats() error = %v", err)
	}

	if stats.Flights != 1 {
		t.Fatalf("flights = %d, want 1 after failed replace", stats.Flights)
	}
}

func TestReplaceFlightPerformanceByMonthReplacesMonth(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)

	columns := testFlightColumns()

	initial := [][]string{testFlightRow(testFlightDate20260424, testAirportORD, testAirportBHM, "UA", "4547", "1535")}
	if err := s.ReplaceFlightPerformanceByMonth(ctx, 2026, 4, columns, initial); err != nil {
		t.Fatalf("initial replace error = %v", err)
	}

	replacement := [][]string{testFlightRow(testFlightDate20260430, testAirportLAX, testAirportSFO, "UA", "100", "0900")}
	if err := s.ReplaceFlightPerformanceByMonth(ctx, 2026, 4, columns, replacement); err != nil {
		t.Fatalf("replacement replace error = %v", err)
	}

	oldStats, err := s.RouteStats(ctx, store.RouteStatsFilter{
		Origin:    testAirportORD,
		Dest:      testAirportBHM,
		StartDate: testFlightDate20260424,
		EndDate:   testFlightDate20260424,
	})
	if err != nil {
		t.Fatalf("RouteStats() error = %v", err)
	}

	if oldStats.Flights != 0 {
		t.Fatalf("old flights = %d, want 0 after replace", oldStats.Flights)
	}

	newStats, err := s.RouteStats(ctx, store.RouteStatsFilter{
		Origin:    testAirportLAX,
		Dest:      testAirportSFO,
		StartDate: testFlightDate20260430,
		EndDate:   testFlightDate20260430,
	})
	if err != nil {
		t.Fatalf("RouteStats() new error = %v", err)
	}

	if newStats.Flights != 1 {
		t.Fatalf("new flights = %d, want 1", newStats.Flights)
	}
}
