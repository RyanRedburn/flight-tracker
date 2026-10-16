package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func (s *Store) SearchAirports(ctx context.Context, query string, limit int) ([]model.Airport, error) {
	contains, prefix := airportSearchPatterns(query)

	rows, err := s.db.QueryContext(ctx, store.QuerySearchAirports, query, contains, prefix, limit)
	if err != nil {
		return nil, fmt.Errorf("search airports: %w", err)
	}

	found, err := scanAirports(rows)
	if err != nil {
		return nil, fmt.Errorf("search airports: %w", err)
	}

	if len(found) > 0 || !looksLikeIATA(query) {
		return found, nil
	}

	rows, err = s.db.QueryContext(ctx, store.QuerySearchSimilarAirportCodes, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search similar airport codes: %w", err)
	}

	similar, err := scanAirports(rows)
	if err != nil {
		return nil, fmt.Errorf("search similar airport codes: %w", err)
	}

	return similar, nil
}

func scanAirports(rows *sql.Rows) ([]model.Airport, error) {
	defer rows.Close()

	out := make([]model.Airport, 0)

	for rows.Next() {
		var (
			airport      model.Airport
			municipality sql.NullString
		)

		if err := rows.Scan(
			&airport.IATACode,
			&airport.Name,
			&municipality,
			&airport.ISOCountry,
			&airport.ISORegion,
			&airport.Type,
			&airport.ScheduledService,
			&airport.Match,
		); err != nil {
			return nil, err
		}

		airport.IATACode = strings.ToUpper(strings.TrimSpace(airport.IATACode))
		airport.Municipality = strings.TrimSpace(municipality.String)
		out = append(out, airport)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func airportSearchPatterns(query string) (contains, prefix string) {
	escaped := escapeLike(query)
	return "%" + escaped + "%", escaped + "%"
}

func escapeLike(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for _, r := range s {
		if r == '\\' || r == '%' || r == '_' {
			b.WriteByte('\\')
		}

		b.WriteRune(r)
	}

	return b.String()
}

func looksLikeIATA(q string) bool {
	if utf8.RuneCountInString(q) != 3 || len(q) != 3 {
		return false
	}

	for i := range 3 {
		c := q[i]
		if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
			return false
		}
	}

	return true
}
