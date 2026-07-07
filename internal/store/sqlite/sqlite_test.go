//go:build cgo

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	_ "github.com/mattn/go-sqlite3"
)

func TestOpenCRUD(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := Open(ctx, "file:"+filepath.ToSlash(dbPath), filepath.Join("..", "..", "..", "migrations", "sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	now := time.Now().UTC().Truncate(time.Second)
	job := &model.Job{
		ID:        "sqlite-job-1",
		Type:      model.JobTypeFetchFlights,
		Status:    model.JobStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.CreateJob(ctx, job); err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	got, err := s.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if got.Type != job.Type {
		t.Errorf("Type = %q, want %q", got.Type, job.Type)
	}

	got.Status = model.JobStatusCompleted
	got.Result = json.RawMessage(`[{"callsign":"X"}]`)
	got.UpdatedAt = now.Add(time.Minute)
	if err := s.UpdateJob(ctx, got); err != nil {
		t.Fatalf("UpdateJob() error = %v", err)
	}

	updated, err := s.GetJob(ctx, job.ID)
	if err != nil {
		t.Fatalf("GetJob() after update error = %v", err)
	}
	if updated.Status != model.JobStatusCompleted {
		t.Errorf("Status = %q, want completed", updated.Status)
	}

	jobs, err := s.ListJobs(ctx, 10)
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("len(jobs) = %d, want 1", len(jobs))
	}
}

func TestGetJobNotFound(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := Open(ctx, "file:"+filepath.ToSlash(dbPath), filepath.Join("..", "..", "..", "migrations", "sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	_, err = s.GetJob(ctx, "missing")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("GetJob() error = %v, want ErrNotFound", err)
	}
}

func TestMigrationVersion(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := Open(ctx, "file:"+filepath.ToSlash(dbPath), filepath.Join("..", "..", "..", "migrations", "sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	version, err := s.MigrationVersion(ctx)
	if err != nil {
		t.Fatalf("MigrationVersion() error = %v", err)
	}

	if version.Version != 3 {
		t.Errorf("Version = %d, want 3", version.Version)
	}

	if version.Dirty {
		t.Error("Dirty = true, want false")
	}
}

func TestSQLiteDBPath(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		want    string
		wantErr bool
	}{
		{name: "file dsn", dsn: "file:/data/app.db", want: filepath.FromSlash("/data/app.db")},
		{name: "plain path", dsn: "app.db", want: "app.db"},
		{name: "sqlite3 url", dsn: "sqlite3://./app.db", want: filepath.FromSlash("./app.db")},
		{name: "empty", dsn: "", wantErr: true},
		{name: "invalid file", dsn: "file:", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sqliteDBPath(tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Fatal("sqliteDBPath() expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("sqliteDBPath() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("sqliteDBPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestListOnTimeFlights(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	migrationsPath := filepath.Join("..", "..", "..", "migrations", "sqlite")

	s, err := Open(ctx, "file:"+filepath.ToSlash(dbPath), migrationsPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer s.Close()

	seedOnTimeFlights(t, dbPath,
		[]string{"2026-04-24", "ORD", "BHM", "UA", "4547", "1535"},
		[]string{"2026-04-24", "ORD", "AVP", "UA", "4546", "1805"},
		[]string{"2026-04-25", "LAX", "SFO", "UA", "100", "0900"},
	)

	flights, err := s.ListOnTimeFlights(ctx, store.OnTimeFlightFilter{
		FlightDate: "2026-04-24",
		Origin:     "ORD",
		Dest:       "BHM",
	})
	if err != nil {
		t.Fatalf("ListOnTimeFlights() error = %v", err)
	}

	if len(flights) != 1 {
		t.Fatalf("len(flights) = %d, want 1", len(flights))
	}

	if flights[0].Flight_Number_Marketing_Airline != "4547" {
		t.Errorf("Flight_Number_Marketing_Airline = %q, want 4547", flights[0].Flight_Number_Marketing_Airline)
	}

	all, err := s.ListOnTimeFlights(ctx, store.OnTimeFlightFilter{Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("ListOnTimeFlights() paginated error = %v", err)
	}

	if len(all) != 2 {
		t.Fatalf("len(all) = %d, want 2", len(all))
	}

	if all[0].Origin != "ORD" {
		t.Errorf("Origin = %q, want ORD", all[0].Origin)
	}
}

func seedOnTimeFlights(t *testing.T, dbPath string, rows ...[]string) {
	t.Helper()

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	const insert = `
		INSERT INTO on_time_flights (
			FlightDate, Origin, Dest,
			IATA_Code_Marketing_Airline, Flight_Number_Marketing_Airline,
			CRSDepTime
		) VALUES (?, ?, ?, ?, ?, ?)`

	for _, row := range rows {
		if _, err := db.Exec(insert, row[0], row[1], row[2], row[3], row[4], row[5]); err != nil {
			t.Fatalf("insert flight: %v", err)
		}
	}
}
