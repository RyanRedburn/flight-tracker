package mem

import (
	"context"
	"errors"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

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

	columns := []string{"FlightDate", "Origin", "Dest"}
	rows := [][]string{{"2026-05-01", "ORD", "BHM"}}
	if err := s.ReplaceOnTimeFlightsByMonth(ctx, 2026, 5, columns, rows); err != nil {
		t.Fatalf("ReplaceOnTimeFlightsByMonth() error = %v", err)
	}

	flights, err := s.ListOnTimeFlights(ctx, store.OnTimeFlightFilter{FlightDate: "2026-05-01"})
	if err != nil {
		t.Fatalf("ListOnTimeFlights() error = %v", err)
	}

	if len(flights) != 1 {
		t.Fatalf("len(flights) = %d, want 1", len(flights))
	}
}
