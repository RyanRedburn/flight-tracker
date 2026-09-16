package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func TestQueryAdvisoryXactLock(t *testing.T) {
	q := store.QueryAdvisoryXactLock
	if !strings.Contains(q, "pg_advisory_xact_lock") || !strings.Contains(q, "hashtext") {
		t.Fatalf("QueryAdvisoryXactLock = %q, want pg_advisory_xact_lock(hashtext(...))", q)
	}
}

func TestLockIngestJob(t *testing.T) {
	exec := &captureExec{}

	if err := lockIngestJob(context.Background(), exec, model.JobTypeImportFlightPerformance, 2026, 4); err != nil {
		t.Fatalf("lockIngestJob() error = %v", err)
	}

	if exec.query != store.QueryAdvisoryXactLock {
		t.Errorf("query = %q, want %q", exec.query, store.QueryAdvisoryXactLock)
	}

	if len(exec.args) != 2 || exec.args[0] != model.JobTypeImportFlightPerformance || exec.args[1] != 202604 {
		t.Errorf("args = %v, want [%q 202604]", exec.args, model.JobTypeImportFlightPerformance)
	}
}

func TestLockIngestJobReference(t *testing.T) {
	exec := &captureExec{}

	if err := lockIngestJob(context.Background(), exec, model.JobTypeImportAirports, 0, 0); err != nil {
		t.Fatalf("lockIngestJob() error = %v", err)
	}

	if len(exec.args) != 2 || exec.args[0] != model.JobTypeImportAirports || exec.args[1] != 0 {
		t.Errorf("args = %v, want [%q 0]", exec.args, model.JobTypeImportAirports)
	}
}

func TestLockIngestJobDistinctMonths(t *testing.T) {
	apr := &captureExec{}
	may := &captureExec{}

	if err := lockIngestJob(context.Background(), apr, model.JobTypeImportWeatherObservations, 2024, 1); err != nil {
		t.Fatalf("April lock: %v", err)
	}

	if err := lockIngestJob(context.Background(), may, model.JobTypeImportWeatherObservations, 2024, 2); err != nil {
		t.Fatalf("May lock: %v", err)
	}

	if apr.args[0] != may.args[0] {
		t.Errorf("type key = %v vs %v, want the same job type", apr.args[0], may.args[0])
	}

	if apr.args[1] == may.args[1] {
		t.Fatalf("month keys collided: %v", apr.args[1])
	}
}

func TestLockIngestJobError(t *testing.T) {
	exec := &captureExec{err: errors.New("db down")}

	err := lockIngestJob(context.Background(), exec, model.JobTypeImportWeatherStations, 0, 0)
	if err == nil {
		t.Fatal("lockIngestJob() error = nil, want wrapped failure")
	}

	if !errors.Is(err, exec.err) {
		t.Errorf("lockIngestJob() error = %v, want wrapped %v", err, exec.err)
	}
}

type captureExec struct {
	query string
	args  []any
	err   error
}

func (c *captureExec) ExecContext(_ context.Context, query string, args ...any) (sql.Result, error) {
	c.query = query
	c.args = args

	if c.err != nil {
		return nil, c.err
	}

	return nopResult{}, nil
}

type nopResult struct{}

func (nopResult) LastInsertId() (int64, error) { return 0, nil }

func (nopResult) RowsAffected() (int64, error) { return 0, nil }
