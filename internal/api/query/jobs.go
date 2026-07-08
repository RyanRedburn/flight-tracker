package query

import (
	"net/http"
)

const (
	DefaultListLimit = 50
	MaxListLimit     = 500
)

type jobsListQuery struct {
	Limit *int `query:"limit" validate:"omitempty,list_limit"`
}

func ParseJobsList(r *http.Request) (int, error) {
	var q jobsListQuery
	if err := BindQuery(r, &q); err != nil {
		return 0, err
	}

	if err := Validate(q); err != nil {
		return 0, err
	}

	if q.Limit == nil {
		return DefaultListLimit, nil
	}

	return *q.Limit, nil
}

type jobIDParam struct {
	ID string `validate:"required"`
}

func ParseJobID(id string) error {
	return Validate(jobIDParam{ID: id})
}
