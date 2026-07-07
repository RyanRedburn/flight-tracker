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
		string(job.Status),
		result,
		errMsg,
		job.UpdatedAt.UTC().Format(time.RFC3339),
		nullTimeString(job.StartedAt),
		nullTimeString(job.EndedAt),
		job.ID,
	)

	return err
}

func (s *Store) ListOnTimeFlights(ctx context.Context, filter store.OnTimeFlightFilter) ([]*model.OnTimeFlight, error) {
	limit := filter.Limit
	offset := filter.Offset

	query := store.QueryListOnTimeFlightsBase
	args := make([]any, 0, 5)
	where := make([]string, 0, 3)

	if filter.FlightDate != "" {
		where = append(where, "flight_date = ?")
		args = append(args, filter.FlightDate)
	}

	if filter.Origin != "" {
		where = append(where, "origin = ?")
		args = append(args, filter.Origin)
	}

	if filter.Dest != "" {
		where = append(where, "dest = ?")
		args = append(args, filter.Dest)
	}

	if len(where) > 0 {
		query += "\n\t\tWHERE " + strings.Join(where, " AND ")
	}

	query += "\n\t\tORDER BY flight_date, origin, crs_dep_time\n\t\tLIMIT ? OFFSET ?"

	args = append(args, limit, offset)

	rows, err := s.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var flights []*model.OnTimeFlight

	for rows.Next() {
		flight, err := scanOnTimeFlight(rows)
		if err != nil {
			return nil, err
		}

		flights = append(flights, flight)
	}

	return flights, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (*model.Job, error) {
	var (
		job       model.Job
		status    string
		result    sql.NullString
		errMsg    sql.NullString
		createdAt string
		updatedAt string
		startedAt sql.NullString
		endedAt   sql.NullString
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

	job.StartedAt, err = parseOptionalTime(startedAt)
	if err != nil {
		return nil, err
	}

	job.EndedAt, err = parseOptionalTime(endedAt)
	if err != nil {
		return nil, err
	}

	return &job, nil
}

func scanOnTimeFlight(row rowScanner) (*model.OnTimeFlight, error) {
	var (
		flightDate         sql.NullString
		origin             sql.NullString
		dest               sql.NullString
		iataMarketing      sql.NullString
		flightNumMarketing sql.NullString
		iataOperating      sql.NullString
		flightNumOperating sql.NullString
		crsDep             sql.NullString
		depTime            sql.NullString
		depDelay           sql.NullString
		crsArr             sql.NullString
		arrTime            sql.NullString
		arrDelay           sql.NullString
		cancelled          sql.NullString
		diverted           sql.NullString
		distance           sql.NullString
	)

	if err := row.Scan(
		&flightDate,
		&origin,
		&dest,
		&iataMarketing,
		&flightNumMarketing,
		&iataOperating,
		&flightNumOperating,
		&crsDep,
		&depTime,
		&depDelay,
		&crsArr,
		&arrTime,
		&arrDelay,
		&cancelled,
		&diverted,
		&distance,
	); err != nil {
		return nil, err
	}

	return &model.OnTimeFlight{
		FlightDate:                      nullString(flightDate),
		Origin:                          nullString(origin),
		Dest:                            nullString(dest),
		IATA_Code_Marketing_Airline:     nullString(iataMarketing),
		Flight_Number_Marketing_Airline: nullString(flightNumMarketing),
		IATA_Code_Operating_Airline:     nullString(iataOperating),
		Flight_Number_Operating_Airline: nullString(flightNumOperating),
		CRSDepTime:                      nullString(crsDep),
		DepTime:                         nullString(depTime),
		DepDelay:                        nullString(depDelay),
		CRSArrTime:                      nullString(crsArr),
		ArrTime:                         nullString(arrTime),
		ArrDelay:                        nullString(arrDelay),
		Cancelled:                       nullString(cancelled),
		Diverted:                        nullString(diverted),
		Distance:                        nullString(distance),
	}, nil
}

func nullString(s sql.NullString) string {
	if s.Valid {
		return s.String
	}

	return ""
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
