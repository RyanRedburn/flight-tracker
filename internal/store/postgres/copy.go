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

	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func (s *Store) ReplaceFlightPerformanceByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error {
	return s.replaceTable(ctx, store.QueryDeleteFlightPerformanceByMonth, []any{
		strconv.Itoa(year),
		strconv.Itoa(month),
	}, "flight_performance", columns, rows)
}

func (s *Store) ReplaceWeatherObservationsByMonth(ctx context.Context, year, month int, columns []string, rows [][]string) error {
	return s.replaceTable(ctx, store.QueryDeleteWeatherObservationsByMonth, []any{
		strconv.Itoa(year),
		strconv.Itoa(month),
	}, "weather_observations", columns, rows)
}

func (s *Store) replaceTable(
	ctx context.Context,
	deleteQuery string,
	deleteArgs []any,
	table string,
	columns []string,
	rows [][]string,
) error {
	if len(columns) == 0 {
		return errors.New("columns required")
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

	if _, err := tx.ExecContext(ctx, deleteQuery, deleteArgs...); err != nil {
		return fmt.Errorf("delete rows: %w", err)
	}

	if len(rows) > 0 {
		if err := copyTableRows(ctx, conn, table, columns, rows); err != nil {
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
