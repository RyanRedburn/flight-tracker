package postgres

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

type tableReplace struct {
	deleteQuery string
	deleteArgs  []any
	table       string
	columns     []string
	rows        [][]string
	// lockKey and lockID match queueing: job type plus year*100+month.
	// Empty lockKey skips the lock. Held only for this delete+COPY transaction.
	lockKey string
	lockID  int
}

func (s *Store) ReplaceFlightPerformanceByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error {
	return s.replaceTables(ctx, tableReplace{
		deleteQuery: store.QueryDeleteFlightPerformanceByMonth,
		deleteArgs: []any{
			strconv.Itoa(year),
			strconv.Itoa(month),
		},
		table:   "flight_performance",
		columns: columns,
		rows:    rows,
		lockKey: string(model.JobTypeImportFlightPerformance),
		lockID:  year*100 + month,
	})
}

func (s *Store) ReplaceWeatherObservationsByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error {
	return s.replaceTables(ctx, tableReplace{
		deleteQuery: store.QueryDeleteWeatherObservationsByMonth,
		deleteArgs: []any{
			strconv.Itoa(year),
			strconv.Itoa(month),
		},
		table:   "weather_observations",
		columns: columns,
		rows:    rows,
		lockKey: string(model.JobTypeImportWeatherObservations),
		lockID:  year*100 + month,
	})
}

func (s *Store) replaceTables(ctx context.Context, ops ...tableReplace) error {
	if len(ops) == 0 {
		return errors.New("replace operations required")
	}

	for _, op := range ops {
		if len(op.columns) == 0 {
			return errors.New("columns required")
		}
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Close()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, op := range ops {
		if op.lockKey == "" {
			continue
		}

		if _, err := tx.ExecContext(ctx, store.QueryAdvisoryXactLock, op.lockKey, op.lockID); err != nil {
			return fmt.Errorf("advisory lock: %w", err)
		}
	}

	for _, op := range ops {
		if _, err := tx.ExecContext(ctx, op.deleteQuery, op.deleteArgs...); err != nil {
			return fmt.Errorf("delete rows: %w", err)
		}
	}

	for _, op := range ops {
		if len(op.rows) == 0 {
			continue
		}

		if err := copyTableRows(ctx, conn, op.table, op.columns, op.rows); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	return nil
}

func copyTableRows(ctx context.Context, conn *sql.Conn, table string, columns []string, rows [][]string) error {
	copySQL, err := buildCopySQL(table, columns)
	if err != nil {
		return err
	}

	return conn.Raw(func(driverConn any) error {
		stdConn, ok := driverConn.(*stdlib.Conn)
		if !ok {
			return fmt.Errorf("unexpected driver connection %T", driverConn)
		}

		pr, pw := io.Pipe()
		errCh := make(chan error, 1)

		go func() {
			errCh <- writeCopyCSV(pw, columns, rows)
		}()

		_, copyErr := stdConn.Conn().PgConn().CopyFrom(ctx, pr, copySQL)
		_ = pr.Close()
		writeErr := <-errCh

		if copyErr != nil {
			return fmt.Errorf("copy %s: %w", table, copyErr)
		}

		if writeErr != nil {
			return fmt.Errorf("copy %s: %w", table, writeErr)
		}

		return nil
	})
}

func buildCopySQL(table string, columns []string) (string, error) {
	if len(columns) == 0 {
		return "", errors.New("columns required")
	}

	quoted := make([]string, len(columns))
	for i, col := range columns {
		quoted[i] = pgx.Identifier{col}.Sanitize()
	}

	return fmt.Sprintf(
		"COPY %s (%s) FROM STDIN WITH (FORMAT csv)",
		pgx.Identifier{table}.Sanitize(),
		strings.Join(quoted, ", "),
	), nil
}

func writeCopyCSV(pw *io.PipeWriter, columns []string, rows [][]string) error {
	err := encodeCopyCSV(pw, columns, rows)
	if err != nil {
		_ = pw.CloseWithError(err)

		return err
	}

	return pw.Close()
}

func encodeCopyCSV(w io.Writer, columns []string, rows [][]string) error {
	csvW := csv.NewWriter(w)

	for _, row := range rows {
		if len(row) != len(columns) {
			return fmt.Errorf("row width %d does not match columns %d", len(row), len(columns))
		}

		if err := csvW.Write(row); err != nil {
			return err
		}
	}

	csvW.Flush()

	return csvW.Error()
}
