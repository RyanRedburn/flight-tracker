package sqlite

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
)

const bulkReplaceBatchSize = 500

func replaceTableRows(ctx context.Context, tx *sqlx.Tx, table string, columns []string, rows [][]string) error {
	if len(columns) == 0 {
		return errors.New("columns required")
	}

	if len(rows) == 0 {
		return nil
	}

	quotedCols := make([]string, len(columns))
	for i, col := range columns {
		quotedCols[i] = quoteSQLiteIdent(col)
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(columns)), ",")
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		quoteSQLiteIdent(table),
		strings.Join(quotedCols, ", "),
		placeholders,
	)

	for start := 0; start < len(rows); start += bulkReplaceBatchSize {
		end := min(start+bulkReplaceBatchSize, len(rows))

		for _, row := range rows[start:end] {
			if len(row) != len(columns) {
				return fmt.Errorf("row width %d does not match columns %d", len(row), len(columns))
			}

			args := make([]any, len(row))
			for i, v := range row {
				args[i] = v
			}

			if _, err := tx.ExecContext(ctx, insertSQL, args...); err != nil {
				return fmt.Errorf("insert row: %w", err)
			}
		}
	}

	return nil
}
