package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func (s *Store) RouteCarriers(ctx context.Context, filter store.RouteCarriersFilter) (*store.RouteCarriers, error) {
	if err := s.ensureRouteIdentity(ctx, filter.Origin, filter.Dest, ""); err != nil {
		return nil, err
	}

	start, end := filter.StartDate, filter.EndDate
	if start == "" && end == "" {
		var maxDate sql.NullString

		err := s.db.QueryRowContext(ctx, store.QueryRouteMaxFlightDate, filter.Origin, filter.Dest).Scan(&maxDate)
		if err != nil {
			return nil, err
		}

		if !maxDate.Valid || maxDate.String == "" {
			return nil, fmt.Errorf("route carriers %s-%s: %w", filter.Origin, filter.Dest, store.ErrNotFound)
		}

		var ok bool

		start, end, ok = store.RouteCarrierWindow(maxDate.String)
		if !ok {
			return nil, fmt.Errorf("parse route max flight date %q", maxDate.String)
		}
	}

	rows, err := s.db.QueryContext(ctx, store.QueryRouteCarriers, filter.Origin, filter.Dest, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	carriers := make([]model.CarrierOnTime, 0)

	for rows.Next() {
		var (
			carrier string
			flights int
			onTime  int
		)

		if err := rows.Scan(&carrier, &flights, &onTime); err != nil {
			return nil, err
		}

		carriers = append(carriers, store.CarrierOnTimeFromCounts(carrier, onTime, flights))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	store.SortCarrierOnTime(carriers)

	rounded := &model.RouteStats{CarrierOnTime: carriers}
	rounded.RoundForResponse()

	if rounded.CarrierOnTime == nil {
		rounded.CarrierOnTime = []model.CarrierOnTime{}
	}

	return &store.RouteCarriers{
		Start:    start,
		End:      end,
		Carriers: rounded.CarrierOnTime,
	}, nil
}
