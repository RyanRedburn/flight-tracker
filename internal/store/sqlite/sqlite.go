package sqlite

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
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sqlx.DB
}

func Open(ctx context.Context, dsn, migrationsPath string) (store.Store, error) {
	dbPath, err := sqliteDBPath(dsn)
	if err != nil {
		return nil, err
	}

	if err := runMigrations(migrationsPath, dbPath); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
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

func (s *Store) CreateJob(ctx context.Context, job *model.Job) error {
	payload := string(job.Payload)
	if payload == "" {
		payload = "{}"
	}

	var result sql.NullString
	if len(job.Result) > 0 {
		result = sql.NullString{String: string(job.Result), Valid: true}
	}

	var errMsg sql.NullString
	if job.Error != "" {
		errMsg = sql.NullString{String: job.Error, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, store.QueryCreateJob,
		job.ID,
		job.Type,
		payload,
		string(job.Status),
		result,
		errMsg,
		job.CreatedAt.UTC().Format(time.RFC3339),
		job.UpdatedAt.UTC().Format(time.RFC3339),
	)

	return err
}

func (s *Store) GetJob(ctx context.Context, id string) (*model.Job, error) {
	row := s.db.QueryRowxContext(ctx, store.QueryGetJob, id)
	job, err := scanJob(row)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("job %q: %w", id, store.ErrNotFound)
	}

	return job, err
}

func (s *Store) ListJobs(ctx context.Context, limit int) ([]*model.Job, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.QueryxContext(ctx, store.QueryListJobs, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*model.Job

	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

func (s *Store) UpdateJob(ctx context.Context, job *model.Job) error {
	payload := string(job.Payload)
	if payload == "" {
		payload = "{}"
	}

	var result sql.NullString
	if len(job.Result) > 0 {
		result = sql.NullString{String: string(job.Result), Valid: true}
	}

	var errMsg sql.NullString
	if job.Error != "" {
		errMsg = sql.NullString{String: job.Error, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, store.QueryUpdateJob,
		job.Type,
		payload,
		string(job.Status),
		result,
		errMsg,
		job.UpdatedAt.UTC().Format(time.RFC3339),
		job.ID,
	)

	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (*model.Job, error) {
	var (
		job       model.Job
		status    string
		payload   string
		result    sql.NullString
		errMsg    sql.NullString
		createdAt string
		updatedAt string
	)

	if err := row.Scan(
		&job.ID,
		&job.Type,
		&payload,
		&status,
		&result,
		&errMsg,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, err
	}

	job.Status = model.JobStatus(status)
	job.Payload = json.RawMessage(payload)

	if result.Valid {
		job.Result = json.RawMessage(result.String)
	}

	if errMsg.Valid {
		job.Error = errMsg.String
	}

	var err error

	job.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}

	job.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &job, nil
}

func runMigrations(migrationsPath, dbPath string) error {
	migrationsURL := toFileURL(migrationsPath)
	databaseURL := "sqlite3://" + filepath.ToSlash(dbPath)

	m, err := migrate.New(migrationsURL, databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}

func sqliteDBPath(dsn string) (string, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return "", errors.New("empty sqlite dsn")
	}

	if strings.HasPrefix(dsn, "file:") {
		path := strings.TrimPrefix(dsn, "file:")

		path = strings.TrimPrefix(path, "//")
		if path == "" {
			return "", errors.New("invalid sqlite file dsn")
		}

		return filepath.FromSlash(path), nil
	}

	if strings.HasPrefix(dsn, "sqlite3://") {
		path := strings.TrimPrefix(dsn, "sqlite3://")
		return filepath.FromSlash(path), nil
	}

	return dsn, nil
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
