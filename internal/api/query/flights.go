package query

import (
	"net/http"
	"strconv"

	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/validation"
)

type flightsListQuery struct {
	FlightDate string `query:"flight_date" validate:"omitempty,datetime=2006-01-02"`
	Origin     string `query:"origin" validate:"omitempty,len=3"`
	Dest       string `query:"dest" validate:"omitempty,len=3"`
	Limit      string `query:"limit" validate:"omitempty,query_int"`
	Offset     string `query:"offset" validate:"omitempty,query_int"`
}

type flightsListPagination struct {
	Limit  int `validate:"list_limit"`
	Offset int `validate:"gte=0"`
}

func ParseFlightsList(r *http.Request) (store.OnTimeFlightFilter, error) {
	q := flightsListQuery{
		FlightDate: r.URL.Query().Get("flight_date"),
		Origin:     r.URL.Query().Get("origin"),
		Dest:       r.URL.Query().Get("dest"),
		Limit:      r.URL.Query().Get("limit"),
		Offset:     r.URL.Query().Get("offset"),
	}

	if err := validation.Validate(q); err != nil {
		return store.OnTimeFlightFilter{}, err
	}

	filter := store.OnTimeFlightFilter{
		FlightDate: q.FlightDate,
		Origin:     q.Origin,
		Dest:       q.Dest,
		Limit:      DefaultListLimit,
	}

	if q.Limit != "" {
		limit, err := strconv.Atoi(q.Limit)
		if err != nil {
			return store.OnTimeFlightFilter{}, err
		}

		filter.Limit = limit
	}

	if q.Offset != "" {
		offset, err := strconv.Atoi(q.Offset)
		if err != nil {
			return store.OnTimeFlightFilter{}, err
		}

		filter.Offset = offset
	}

	pagination := flightsListPagination{
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}

	if err := validation.Validate(pagination); err != nil {
		return store.OnTimeFlightFilter{}, err
	}

	return filter, nil
}
