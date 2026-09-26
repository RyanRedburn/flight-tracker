package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, dsn, migrationsPath string) (store.Store, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("empty postgres dsn")
	}

	if err := runMigrations(migrationsPath, dsn); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *Store) MigrationVersion(ctx context.Context) (store.MigrationVersion, error) {
	var (
		version uint
		dirty   bool
	)

	err := s.db.QueryRowContext(ctx, store.QueryMigrationVersion).Scan(&version, &dirty)
	if errors.Is(err, sql.ErrNoRows) {
		return store.MigrationVersion{}, nil
	}

	if err != nil {
		return store.MigrationVersion{}, err
	}

	return store.MigrationVersion{Version: version, Dirty: dirty}, nil
}

func (s *Store) GetJob(ctx context.Context, id string) (*model.Job, error) {
	row := s.db.QueryRowContext(ctx, store.QueryGetJob, id)
	job, err := scanJob(row)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("job %q: %w", id, store.ErrNotFound)
	}

	return job, err
}

func (s *Store) ListJobs(ctx context.Context, limit int) ([]*model.Job, error) {
	rows, err := s.db.QueryContext(ctx, store.QueryListJobs, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*model.Job

	for rows.Next() {
		job, err := scanListedJob(rows)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

type scannedJob struct {
	job       model.Job
	status    string
	result    []byte
	errMsg    sql.NullString
	createdAt time.Time
	updatedAt time.Time
	startedAt sql.NullTime
	endedAt   sql.NullTime
}

func (s *scannedJob) dest() []any {
	return []any{
		&s.job.ID,
		&s.job.Type,
		&s.status,
		&s.result,
		&s.errMsg,
		&s.createdAt,
		&s.updatedAt,
		&s.startedAt,
		&s.endedAt,
	}
}

func (s *scannedJob) model() *model.Job {
	s.job.Status = model.JobStatus(s.status)
	s.job.CreatedAt = s.createdAt.UTC()
	s.job.UpdatedAt = s.updatedAt.UTC()

	if len(s.result) > 0 {
		s.job.Result = json.RawMessage(s.result)
	}

	if s.errMsg.Valid {
		s.job.Error = s.errMsg.String
	}

	if s.startedAt.Valid {
		t := s.startedAt.Time.UTC()
		s.job.StartedAt = &t
	}

	if s.endedAt.Valid {
		t := s.endedAt.Time.UTC()
		s.job.EndedAt = &t
	}

	return &s.job
}

func scanJob(row rowScanner) (*model.Job, error) {
	var scanned scannedJob
	if err := row.Scan(scanned.dest()...); err != nil {
		return nil, err
	}

	return scanned.model(), nil
}

func scanListedJob(row rowScanner) (*model.Job, error) {
	var (
		scanned  scannedJob
		year     sql.NullInt64
		month    sql.NullInt64
		stations []byte
	)

	if err := row.Scan(append(scanned.dest(), &year, &month, &stations)...); err != nil {
		return nil, err
	}

	job := scanned.model()
	ingest := &model.JobListIngest{}

	if year.Valid {
		y := int(year.Int64)
		ingest.Year = &y
	}

	if month.Valid {
		m := int(month.Int64)
		ingest.Month = &m
	}

	if len(stations) > 0 {
		if err := json.Unmarshal(stations, &ingest.Stations); err != nil {
			return nil, fmt.Errorf("unmarshal stations: %w", err)
		}
	}

	job.ListIngest = ingest

	return job, nil
}

func nullJSON(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}

	return []byte(b)
}

func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}

	return sql.NullTime{Time: t.UTC(), Valid: true}
}

func runMigrations(migrationsPath, databaseURL string) error {
	migrationsURL := toFileURL(migrationsPath)

	m, err := migrate.New(migrationsURL, toPgx5URL(databaseURL))
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}

func toFileURL(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") && !strings.Contains(path, ":/") {
		abs, err := filepath.Abs(path)
		if err == nil {
			path = filepath.ToSlash(abs)
		}
	}

	return "file://" + path
}

func toPgx5URL(databaseURL string) string {
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if rest, ok := strings.CutPrefix(databaseURL, prefix); ok {
			return "pgx5://" + rest
		}
	}

	return databaseURL
}
