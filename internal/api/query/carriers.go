package query

import (
	"net/http"
	"strconv"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/go-playground/validator/v10"
)

type carrierStatsQuery struct {
	Carrier   string    `query:"carrier" validate:"required,len=2"`
	StartDate time.Time `query:"start_date" validate:"required_with=EndDate"`
	EndDate   time.Time `query:"end_date" validate:"required_with=StartDate,omitempty,gtefield=StartDate"`
	State     string    `query:"state" validate:"omitempty,len=2"`
}

func init() {
	validate.RegisterStructValidation(validateCarrierStatsSpan, carrierStatsQuery{})
}

func ParseCarrierStats(r *http.Request) (store.CarrierStatsFilter, error) {
	var q carrierStatsQuery
	if err := BindQuery(r, &q); err != nil {
		return store.CarrierStatsFilter{}, err
	}

	normalizeQueryStrings(&q)

	if err := Validate(q); err != nil {
		return store.CarrierStatsFilter{}, err
	}

	filter := store.CarrierStatsFilter{
		Carrier: q.Carrier,
		State:   q.State,
	}

	if !q.StartDate.IsZero() {
		filter.StartDate = q.StartDate.Format("2006-01-02")
		filter.EndDate = q.EndDate.Format("2006-01-02")
	}

	return filter, nil
}

func validateCarrierStatsSpan(sl validator.StructLevel) {
	q := sl.Current().Interface().(carrierStatsQuery)
	if q.StartDate.IsZero() || q.EndDate.IsZero() || q.EndDate.Before(q.StartDate) {
		return
	}

	spanDays := int(q.EndDate.Sub(q.StartDate).Hours()/24) + 1
	if spanDays > store.MaxStatsSpanDays {
		sl.ReportError(q.EndDate, "EndDate", "end_date", "date_span", strconv.Itoa(store.MaxStatsSpanDays))
	}
}
