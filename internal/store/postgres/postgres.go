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
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
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
	where := make([]string, 0, 6)
	n := 1

	where = append(where, fmt.Sprintf("origin = $%d", n))
	args = append(args, origin)
	n++

	where = append(where, fmt.Sprintf("dest = $%d", n))
	args = append(args, dest)
	n++

	if startDate != "" {
		where = append(where, fmt.Sprintf("flight_date >= $%d", n))
		args = append(args, startDate)
		n++
	}

	if endDate != "" {
		where = append(where, fmt.Sprintf("flight_date <= $%d", n))
		args = append(args, endDate)
		n++
	}

	if carrier != "" {
		where = append(where, fmt.Sprintf("iata_code_marketing_airline = $%d", n))
		args = append(args, carrier)
		n++
	}

	if flightNumber != "" {
		where = append(where, fmt.Sprintf("flight_number_marketing_airline = $%d", n))
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
