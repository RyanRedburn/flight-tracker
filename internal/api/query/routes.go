package query

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store"

	"github.com/go-playground/validator/v10"
)

type routeStatsQuery struct {
	Origin       string    `query:"origin" validate:"required,len=3"`
	Dest         string    `query:"dest" validate:"required,len=3"`
	StartDate    time.Time `query:"start_date" validate:"required"`
	EndDate      time.Time `query:"end_date" validate:"required,gtefield=StartDate"`
	Carrier      string    `query:"carrier" validate:"required_with=FlightNumber,omitempty,len=2"`
	FlightNumber string    `query:"flight_number"`
	DaysOfWeek   []int     `query:"days_of_week" validate:"dive,gte=1,lte=7"`
}

type routeOutlookQuery struct {
	Origin               string `query:"origin" validate:"required,len=3"`
	Dest                 string `query:"dest" validate:"required,len=3"`
	Carrier              string `query:"carrier" validate:"required,len=2"`
	DayOfWeek            int    `query:"day_of_week" validate:"required,gte=1,lte=7"`
	DepTime              string `query:"dep_time" validate:"required,hhmm"`
	DepTimeWindowMinutes *int   `query:"dep_time_window_minutes" validate:"omitempty,gte=1,lte=120"`
}

func init() {
	_ = validate.RegisterValidation("hhmm", validateHHMM)
	validate.RegisterStructValidation(validateRouteStatsSpan, routeStatsQuery{})
}

func ParseRouteStats(r *http.Request) (store.RouteStatsFilter, error) {
	var q routeStatsQuery
	if err := BindQuery(r, &q); err != nil {
		return store.RouteStatsFilter{}, err
	}

	normalizeQueryStrings(&q)

	if err := Validate(q); err != nil {
		return store.RouteStatsFilter{}, err
	}

	return store.RouteStatsFilter{
		Origin:       q.Origin,
		Dest:         q.Dest,
		StartDate:    q.StartDate.Format("2006-01-02"),
		EndDate:      q.EndDate.Format("2006-01-02"),
		Carrier:      q.Carrier,
		FlightNumber: q.FlightNumber,
		DaysOfWeek:   uniqueDays(q.DaysOfWeek),
	}, nil
}

func ParseRouteOutlook(r *http.Request) (store.RouteOutlookFilter, error) {
	var q routeOutlookQuery
	if err := BindQuery(r, &q); err != nil {
		return store.RouteOutlookFilter{}, err
	}

	normalizeQueryStrings(&q)

	if err := Validate(q); err != nil {
		return store.RouteOutlookFilter{}, err
	}

	window := store.DefaultDepTimeWindowMinutes
	if q.DepTimeWindowMinutes != nil {
		window = *q.DepTimeWindowMinutes
	}

	return store.RouteOutlookFilter{
		Origin:               q.Origin,
		Dest:                 q.Dest,
		Carrier:              q.Carrier,
		DayOfWeek:            q.DayOfWeek,
		DepTime:              padHHMM(q.DepTime),
		DepTimeWindowMinutes: window,
	}, nil
}

func validateRouteStatsSpan(sl validator.StructLevel) {
	q := sl.Current().Interface().(routeStatsQuery)
	if q.StartDate.IsZero() || q.EndDate.IsZero() || q.EndDate.Before(q.StartDate) {
		return
	}

	spanDays := int(q.EndDate.Sub(q.StartDate).Hours()/24) + 1
	if spanDays > store.MaxStatsSpanDays {
		sl.ReportError(q.EndDate, "EndDate", "end_date", "date_span", strconv.Itoa(store.MaxStatsSpanDays))
	}
}

func validateHHMM(fl validator.FieldLevel) bool {
	_, ok := store.HHMMToMinutes(fl.Field().String())
	return ok
}

func uniqueDays(days []int) []int {
	if len(days) == 0 {
		return []int{}
	}

	out := make([]int, 0, len(days))

	seen := make(map[int]struct{}, len(days))
	for _, day := range days {
		if _, ok := seen[day]; ok {
			continue
		}

		seen[day] = struct{}{}
		out = append(out, day)
	}

	return out
}

func padHHMM(raw string) string {
	n, err := strconv.Atoi(raw)
	if err != nil {
		return raw
	}

	return fmt.Sprintf("%04d", n)
}
