package query

import (
	"net/http"
	"strconv"

	"github.com/RyanRedburn/flight-tracker/internal/validation"
)

type jobsListQuery struct {
	Limit string `query:"limit" validate:"omitempty,query_int"`
}

type jobsListPagination struct {
	Limit int `validate:"list_limit"`
}

func ParseJobsList(r *http.Request) (int, error) {
	q := jobsListQuery{
		Limit: r.URL.Query().Get("limit"),
	}

	if err := validation.Validate(q); err != nil {
		return 0, err
	}

	limit := DefaultListLimit

	if q.Limit != "" {
		parsed, err := strconv.Atoi(q.Limit)
		if err != nil {
			return 0, err
		}

		limit = parsed
	}

	if err := validation.Validate(jobsListPagination{Limit: limit}); err != nil {
		return 0, err
	}

	return limit, nil
}

type jobIDParam struct {
	ID string `validate:"required"`
}

func ParseJobID(id string) error {
	return validation.Validate(jobIDParam{ID: id})
}
