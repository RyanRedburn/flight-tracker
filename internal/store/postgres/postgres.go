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
	"github.com/jmoiron/sqlx"
)

type Store struct {
	db *sqlx.DB
}

func Open(ctx context.Context, dsn, migrationsPath string) (store.Store, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return nil, errors.New("empty postgres dsn")
	}

	if err := runMigrations(migrationsPath, dsn); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	db, err := sqlx.Open("pgx", dsn)
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

func (s *Store) CreateJob(ctx context.Context, job *model.Job) error {
	return execCreateJob(ctx, s.db, job)
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
	var errMsg sql.NullString
	if job.Error != "" {
		errMsg = sql.NullString{String: job.Error, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, store.QueryUpdateJob,
		job.Type,
		string(job.Status),
		nullJSON(job.Result),
		errMsg,
		job.UpdatedAt.UTC(),
		nullTime(job.StartedAt),
		nullTime(job.EndedAt),
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
		result    []byte
		errMsg    sql.NullString
		createdAt time.Time
		updatedAt time.Time
		startedAt sql.NullTime
		endedAt   sql.NullTime
	)

	if err := row.Scan(
		&job.ID,
		&job.Type,
		&status,
		&result,
		&errMsg,
		&createdAt,
		&updatedAt,
		&startedAt,
		&endedAt,
	); err != nil {
		return nil, err
	}

	job.Status = model.JobStatus(status)
	job.CreatedAt = createdAt.UTC()
	job.UpdatedAt = updatedAt.UTC()

	if len(result) > 0 {
		job.Result = json.RawMessage(result)
	}

	if errMsg.Valid {
		job.Error = errMsg.String
	}

	if startedAt.Valid {
		t := startedAt.Time.UTC()
		job.StartedAt = &t
	}

	if endedAt.Valid {
		t := endedAt.Time.UTC()
		job.EndedAt = &t
	}

	return &job, nil
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
