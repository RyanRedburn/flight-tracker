package mem

import (
	"context"
	"errors"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

const testFlightDate20260501 = "2026-05-01"

func TestMemCreateBTSIngestJobAndClaim(t *testing.T) {
	ctx := context.Background()
	s := New()

	job, err := s.CreateBTSIngestJob(ctx, 2026, 6)
	if err != nil {
		t.Fatalf("CreateBTSIngestJob() error = %v", err)
	}

	detail, err := s.GetBTSIngestJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetBTSIngestJob() error = %v", err)
	}

	if detail.Month != 6 {
		t.Errorf("Month = %d, want 6", detail.Month)
	}

	claimed, err := s.ClaimNextPendingJob(ctx)
	if err != nil {
		t.Fatalf("ClaimNextPendingJob() error = %v", err)
	}

	if claimed.ID != job.ID {
		t.Errorf("claimed ID = %q, want %q", claimed.ID, job.ID)
	}
}

func TestMemActiveBTSIngestMonths(t *testing.T) {
	ctx := context.Background()
	s := New()

	if _, err := s.CreateBTSIngestJob(ctx, 2026, 1); err != nil {
		t.Fatalf("CreateBTSIngestJob() error = %v", err)
	}

	active, err := s.ActiveBTSIngestMonths(ctx, []model.YearMonth{{Year: 2026, Month: 1}})
	if err != nil {
		t.Fatalf("ActiveBTSIngestMonths() error = %v", err)
	}

	if len(active) != 1 {
		t.Fatalf("len(active) = %d, want 1", len(active))
	}
}

func TestMemActiveIngestJob(t *testing.T) {
	ctx := context.Background()
	s := New()

	active, err := s.ActiveIngestJob(ctx, model.JobTypeImportBTSOnTime)
	if err != nil {
		t.Fatalf("ActiveIngestJob() error = %v", err)
	}

	if active {
		t.Fatal("expected no active job before create")
	}

	if _, err := s.CreateBTSIngestJob(ctx, 2026, 1); err != nil {
		t.Fatalf("CreateBTSIngestJob() error = %v", err)
	}

	active, err = s.ActiveIngestJob(ctx, model.JobTypeImportBTSOnTime)
	if err != nil {
		t.Fatalf("ActiveIngestJob() error = %v", err)
	}

	if !active {
		t.Fatal("expected active job after create")
	}

	active, err = s.ActiveIngestJob(ctx, "other_job_type")
	if err != nil {
		t.Fatalf("ActiveIngestJob() error = %v", err)
	}

	if active {
		t.Fatal("expected no active job for unrelated type")
	}
}

func TestMemActiveIngestJobOurAirports(t *testing.T) {
	ctx := context.Background()
	s := New()

	if _, err := s.CreateOurAirportsIngestJob(ctx, model.JobTypeImportOurAirportsCountries); err != nil {
		t.Fatalf("CreateOurAirportsIngestJob() error = %v", err)
	}

	active, err := s.ActiveIngestJob(ctx, model.JobTypeImportOurAirportsCountries)
	if err != nil {
		t.Fatalf("ActiveIngestJob() error = %v", err)
	}

	if !active {
		t.Fatal("expected active ourairports countries job")
	}

	active, err = s.ActiveIngestJob(ctx, model.JobTypeImportOurAirportsRegions)
	if err != nil {
		t.Fatalf("ActiveIngestJob() error = %v", err)
	}

	if active {
		t.Fatal("expected no active regions job when only countries is pending")
	}
}

func TestMemCompleteJobStatusConflict(t *testing.T) {
	ctx := context.Background()
	s := New()

	job, err := s.CreateBTSIngestJob(ctx, 2026, 1)
	if err != nil {
		t.Fatalf("CreateBTSIngestJob() error = %v", err)
	}

	if err := s.CompleteJob(ctx, job.ID, []byte(`{}`)); !errors.Is(err, store.ErrJobStatusConflict) {
		t.Fatalf("CompleteJob() without claim error = %v, want ErrJobStatusConflict", err)
	}
}

func TestMemReplaceOnTimeFlightsByMonth(t *testing.T) {
	ctx := context.Background()
	s := New()

	columns := []string{"flight_date", "origin", "dest", "iata_code_marketing_airline"}

	rows := [][]string{{testFlightDate20260501, "ORD", "BHM", "UA"}}
	if err := s.ReplaceOnTimeFlightsByMonth(ctx, 2026, 5, columns, rows); err != nil {
		t.Fatalf("ReplaceOnTimeFlightsByMonth() error = %v", err)
	}

	stats, err := s.RouteStats(ctx, store.RouteStatsFilter{
		Origin:    "ORD",
		Dest:      "BHM",
		StartDate: testFlightDate20260501,
		EndDate:   testFlightDate20260501,
	})
	if err != nil {
		t.Fatalf("RouteStats() error = %v", err)
	}

	if stats.Flights != 1 {
		t.Fatalf("flights = %d, want 1", stats.Flights)
	}
}
