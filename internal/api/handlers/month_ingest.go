package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/ingest"
	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type monthIngestInput struct {
	startYear  int
	startMonth int
	endYear    *int
	endMonth   *int
	force      bool
}

type queuedMonthJob struct {
	ID     string
	Year   int
	Month  int
	Status model.JobStatus
}

// queueMonthIngestJobs expands a month range and queues one job per month.
// Each create call commits on its own. A later 409 or 500 leaves earlier months queued.
// When it writes an error response it returns false.
func queueMonthIngestJobs(
	w http.ResponseWriter,
	r *http.Request,
	maxMonths int,
	in monthIngestInput,
	active func(context.Context, []model.YearMonth) ([]model.YearMonth, error),
	existing func(context.Context, []model.YearMonth) ([]model.YearMonth, error),
	existingCheckErr string,
	existingDataErr string,
	create func(context.Context, int, int) (*model.Job, error),
) ([]queuedMonthJob, bool) {
	months, err := ingest.ExpandMonths(ingest.RangeInput{
		StartYear:  in.startYear,
		StartMonth: in.startMonth,
		EndYear:    in.endYear,
		EndMonth:   in.endMonth,
	}, maxMonths)
	if err != nil {
		writeIngestRangeError(w, err)
		return nil, false
	}

	ctx := r.Context()

	activeMonths, err := active(ctx, months)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCheckActiveIngest})
		return nil, false
	}

	if len(activeMonths) > 0 {
		writeJSON(w, http.StatusConflict, MonthIngestConflictResponse{
			Error:              errActiveIngestMonths,
			ActiveIngestMonths: activeMonths,
		})

		return nil, false
	}

	if !in.force {
		existingMonths, err := existing(ctx, months)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: existingCheckErr})
			return nil, false
		}

		if len(existingMonths) > 0 {
			writeJSON(w, http.StatusConflict, MonthIngestConflictResponse{
				Error:              existingDataErr,
				ExistingDataMonths: existingMonths,
			})

			return nil, false
		}
	}

	jobs := make([]queuedMonthJob, 0, len(months))

	for _, ym := range months {
		job, err := create(ctx, ym.Year, ym.Month)
		if err != nil {
			if errors.Is(err, store.ErrActiveIngestConflict) {
				writeJSON(w, http.StatusConflict, MonthIngestConflictResponse{
					Error:              errActiveIngestMonths,
					ActiveIngestMonths: []model.YearMonth{ym},
				})

				return nil, false
			}

			writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedCreateIngestJob})

			return nil, false
		}

		jobs = append(jobs, queuedMonthJob{
			ID:     job.ID,
			Year:   ym.Year,
			Month:  ym.Month,
			Status: job.Status,
		})
	}

	return jobs, true
}
