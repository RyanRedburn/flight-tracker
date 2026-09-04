package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func (s *Store) DistinctFlightAirportCodes(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, store.QueryDistinctFlightAirportCodes)
	if err != nil {
		return nil, fmt.Errorf("query distinct flight airports: %w", err)
	}
	defer rows.Close()

	var codes []string

	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}

		codes = append(codes, code)
	}

	return codes, rows.Err()
}

func (s *Store) ListAirportIdentifiersByIATA(ctx context.Context, codes []string) (map[string]store.AirportIdentifiers, error) {
	out := make(map[string]store.AirportIdentifiers)

	normalized := uniqueUpperCodes(codes)
	if len(normalized) == 0 {
		return out, nil
	}

	rows, err := s.db.QueryContext(ctx, store.QueryListAirportIdentifiersByIATA, normalized)
	if err != nil {
		return nil, fmt.Errorf("query airport identifiers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			iata      string
			localCode sql.NullString
			icaoCode  sql.NullString
			ident     sql.NullString
		)

		if err := rows.Scan(&iata, &localCode, &icaoCode, &ident); err != nil {
			return nil, err
		}

		iata = strings.ToUpper(strings.TrimSpace(iata))
		if iata == "" {
			continue
		}

		if _, exists := out[iata]; exists {
			continue
		}

		out[iata] = store.AirportIdentifiers{
			IATACode:  iata,
			LocalCode: strings.ToUpper(strings.TrimSpace(localCode.String)),
			ICAOCode:  strings.ToUpper(strings.TrimSpace(icaoCode.String)),
			Ident:     strings.ToUpper(strings.TrimSpace(ident.String)),
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func uniqueUpperCodes(codes []string) []string {
	out := make([]string, 0, len(codes))
	seen := make(map[string]struct{}, len(codes))

	for _, code := range codes {
		code = strings.ToUpper(strings.TrimSpace(code))
		if code == "" {
			continue
		}

		if _, ok := seen[code]; ok {
			continue
		}

		seen[code] = struct{}{}
		out = append(out, code)
	}

	return out
}
