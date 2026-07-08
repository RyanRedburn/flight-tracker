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

func (s *Store) RouteStats(ctx context.Context, filter store.RouteStatsFilter) (*model.RouteStats, error) {
	rows, err := s.queryFlightPerf(ctx, filter.Origin, filter.Dest, filter.StartDate, filter.EndDate, filter.Carrier, filter.FlightNumber)
	if err != nil {
		return nil, err
	}

	return store.AggregateRouteStats(filter, rows), nil
}

func (s *Store) RouteOutlook(ctx context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
	rows, err := s.queryFlightPerf(ctx, filter.Origin, filter.Dest, "", "", filter.Carrier, "")
	if err != nil {
		return nil, err
	}

	return store.AggregateRouteOutlook(filter, rows), nil
}

func (s *Store) queryFlightPerf(ctx context.Context, origin, dest, startDate, endDate, carrier, flightNumber string) ([]store.FlightPerf, error) {
	query := store.QueryRoutePerfBase
	args := make([]any, 0, 6)
	where := []string{"origin = ?", "dest = ?"}

	args = append(args, origin, dest)

	if startDate != "" {
		where = append(where, "flight_date >= ?")
		args = append(args, startDate)
	}

	if endDate != "" {
		where = append(where, "flight_date <= ?")
		args = append(args, endDate)
	}

	if carrier != "" {
		where = append(where, "iata_code_marketing_airline = ?")
		args = append(args, carrier)
	}

	if flightNumber != "" {
		where = append(where, "flight_number_marketing_airline = ?")
		args = append(args, flightNumber)
	}

	query += "\n\t\tWHERE " + strings.Join(where, " AND ")

	rows, err := s.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []store.FlightPerf

	for rows.Next() {
		perf, err := scanFlightPerf(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, perf)
	}

	return out, rows.Err()
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

func scanFlightPerf(row rowScanner) (store.FlightPerf, error) {
	var (
		flightDate        sql.NullString
		dayOfWeek         sql.NullString
		origin            sql.NullString
		dest              sql.NullString
		carrier           sql.NullString
		flightNumber      sql.NullString
		crsDep            sql.NullString
		arrDelayMinutes   sql.NullString
		depDelayMinutes   sql.NullString
		arrDel15          sql.NullString
		depDel15          sql.NullString
		cancelled         sql.NullString
		cancellationCode  sql.NullString
		diverted          sql.NullString
		carrierDelay      sql.NullString
		weatherDelay      sql.NullString
		nasDelay          sql.NullString
		securityDelay     sql.NullString
		lateAircraftDelay sql.NullString
		div1              sql.NullString
		div2              sql.NullString
		div3              sql.NullString
		div4              sql.NullString
		div5              sql.NullString
	)

	if err := row.Scan(
		&flightDate,
		&dayOfWeek,
		&origin,
		&dest,
		&carrier,
		&flightNumber,
		&crsDep,
		&arrDelayMinutes,
		&depDelayMinutes,
		&arrDel15,
		&depDel15,
		&cancelled,
		&cancellationCode,
		&diverted,
		&carrierDelay,
		&weatherDelay,
		&nasDelay,
		&securityDelay,
		&lateAircraftDelay,
		&div1,
		&div2,
		&div3,
		&div4,
		&div5,
	); err != nil {
		return store.FlightPerf{}, err
	}

	return store.FlightPerf{
		FlightDate:        nullString(flightDate),
		DayOfWeek:         nullString(dayOfWeek),
		Origin:            nullString(origin),
		Dest:              nullString(dest),
		Carrier:           nullString(carrier),
		FlightNumber:      nullString(flightNumber),
		CRSDepTime:        nullString(crsDep),
		ArrDelayMinutes:   nullString(arrDelayMinutes),
		DepDelayMinutes:   nullString(depDelayMinutes),
		ArrDel15:          nullString(arrDel15),
		DepDel15:          nullString(depDel15),
		Cancelled:         nullString(cancelled),
		CancellationCode:  nullString(cancellationCode),
		Diverted:          nullString(diverted),
		CarrierDelay:      nullString(carrierDelay),
		WeatherDelay:      nullString(weatherDelay),
		NASDelay:          nullString(nasDelay),
		SecurityDelay:     nullString(securityDelay),
		LateAircraftDelay: nullString(lateAircraftDelay),
		DivAirports: [5]string{
			nullString(div1),
			nullString(div2),
			nullString(div3),
			nullString(div4),
			nullString(div5),
		},
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
