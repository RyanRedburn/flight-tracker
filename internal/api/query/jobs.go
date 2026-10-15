package query

import (
	"net/http"
)

const (
	DefaultListLimit = 50
	MaxListLimit     = 500
)

type jobsListQuery struct {
	Limit        *int   `query:"limit" validate:"omitempty,list_limit"`
	OpenJobsOnly string `query:"open_jobs_only" validate:"omitempty,oneof=true false"`
}

// JobsList is the parsed jobs list query. OpenJobsOnly is false when omitted.
type JobsList struct {
	Limit        int
	OpenJobsOnly bool
}

func ParseJobsList(r *http.Request) (JobsList, error) {
	var q jobsListQuery
	if err := BindQuery(r, &q); err != nil {
		return JobsList{}, err
	}

	if err := Validate(q); err != nil {
		return JobsList{}, err
	}

	parsed := JobsList{Limit: DefaultListLimit, OpenJobsOnly: q.OpenJobsOnly == "true"}
	if q.Limit != nil {
		parsed.Limit = *q.Limit
	}

	return parsed, nil
}

type jobIDParam struct {
	ID string `validate:"required"`
}

func ParseJobID(id string) error {
	return Validate(jobIDParam{ID: id})
}
