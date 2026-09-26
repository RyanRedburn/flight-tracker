package postgres

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

type fakeScanner struct {
	values []any
}

func (f fakeScanner) Scan(dest ...any) error {
	if len(dest) != len(f.values) {
		return fmt.Errorf("scan dest %d, values %d", len(dest), len(f.values))
	}

	for i, d := range dest {
		dv := reflect.ValueOf(d)
		if dv.Kind() != reflect.Pointer || dv.IsNil() {
			return fmt.Errorf("dest %d is not a pointer", i)
		}

		src := f.values[i]
		if src == nil {
			dv.Elem().Set(reflect.Zero(dv.Elem().Type()))
			continue
		}

		sv := reflect.ValueOf(src)
		if !sv.Type().AssignableTo(dv.Elem().Type()) {
			return fmt.Errorf("dest %d: cannot assign %T to %s", i, src, dv.Elem().Type())
		}

		dv.Elem().Set(sv)
	}

	return nil
}

func TestScanListedJob(t *testing.T) {
	now := time.Date(2024, 6, 1, 15, 0, 0, 0, time.UTC)
	started := now.Add(time.Minute)

	job, err := scanListedJob(fakeScanner{values: []any{
		"job-weather",
		model.JobTypeImportWeatherObservations,
		string(model.JobStatusRunning),
		[]byte(`{"rows_imported":3}`),
		sql.NullString{},
		now,
		now,
		sql.NullTime{Time: started, Valid: true},
		sql.NullTime{},
		sql.NullInt64{Int64: 2024, Valid: true},
		sql.NullInt64{Int64: 6, Valid: true},
		[]byte(`["ORD","JFK"]`),
	}})
	if err != nil {
		t.Fatalf("scanListedJob() error = %v", err)
	}

	if job.ListIngest == nil || job.ListIngest.Year == nil || *job.ListIngest.Year != 2024 {
		t.Fatalf("year = %#v", job.ListIngest)
	}

	if job.ListIngest.Month == nil || *job.ListIngest.Month != 6 {
		t.Errorf("month = %v, want 6", job.ListIngest.Month)
	}

	if len(job.ListIngest.Stations) != 2 || job.ListIngest.Stations[0] != "ORD" || job.ListIngest.Stations[1] != "JFK" {
		t.Errorf("stations = %v, want [ORD JFK]", job.ListIngest.Stations)
	}

	if job.StartedAt == nil || !job.StartedAt.Equal(started) {
		t.Errorf("started_at = %v, want %v", job.StartedAt, started)
	}

	if string(job.Result) != `{"rows_imported":3}` {
		t.Errorf("result = %s", job.Result)
	}
}

func TestScanListedJobWithoutDetail(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	job, err := scanListedJob(fakeScanner{values: []any{
		"job-rebuild",
		model.JobTypeRebuildRouteTravelWindows,
		string(model.JobStatusPending),
		[]byte(nil),
		sql.NullString{},
		now,
		now,
		sql.NullTime{},
		sql.NullTime{},
		sql.NullInt64{},
		sql.NullInt64{},
		[]byte(nil),
	}})
	if err != nil {
		t.Fatalf("scanListedJob() error = %v", err)
	}

	if job.ListIngest == nil {
		t.Fatal("ListIngest is nil")
	}

	if job.ListIngest.Year != nil || job.ListIngest.Month != nil || job.ListIngest.Stations != nil {
		t.Errorf("detail = %#v, want empty", job.ListIngest)
	}
}

func TestScanListedJobBadStations(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	_, err := scanListedJob(fakeScanner{values: []any{
		"job-weather",
		model.JobTypeImportWeatherObservations,
		string(model.JobStatusPending),
		[]byte(nil),
		sql.NullString{},
		now,
		now,
		sql.NullTime{},
		sql.NullTime{},
		sql.NullInt64{Int64: 2024, Valid: true},
		sql.NullInt64{Int64: 1, Valid: true},
		[]byte(`not-json`),
	}})
	if err == nil {
		t.Fatal("scanListedJob() error = nil, want unmarshal error")
	}
}
