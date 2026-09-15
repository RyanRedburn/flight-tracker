package postgres

import (
	"fmt"
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil",
			err:  nil,
			want: false,
		},
		{
			name: "other error",
			err:  fmt.Errorf("boom"),
			want: false,
		},
		{
			name: "unique violation",
			err:  &pgconn.PgError{Code: pgerrcode.UniqueViolation},
			want: true,
		},
		{
			name: "wrapped unique violation",
			err:  fmt.Errorf("insert: %w", &pgconn.PgError{Code: pgerrcode.UniqueViolation}),
			want: true,
		},
		{
			name: "other pg error",
			err:  &pgconn.PgError{Code: pgerrcode.ForeignKeyViolation},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isUniqueViolation(tt.err); got != tt.want {
				t.Errorf("isUniqueViolation() = %v, want %v", got, tt.want)
			}
		})
	}
}
