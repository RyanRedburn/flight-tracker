package query

import (
	"errors"
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
	Origin               string    `query:"origin" validate:"required,len=3"`
	Dest                 string    `query:"dest" validate:"required,len=3"`
	Carrier              string    `query:"carrier" validate:"required,len=2"`
	DayOfWeek            *int      `query:"day_of_week" validate:"omitempty,gte=1,lte=7"`
	Date                 time.Time `query:"date"`
	DepTime              string    `query:"dep_time" validate:"required,hhmm"`
	DepTimeWindowMinutes *int      `query:"dep_time_window_minutes" validate:"omitempty,gte=1,lte=120"`
}

type routeTravelWindowsQuery struct {
	Origin  string `query:"origin" validate:"required,len=3"`
	Dest    string `query:"dest" validate:"required,len=3"`
	Carrier string `query:"carrier" validate:"omitempty,len=2"`
}

func init() {
	_ = validate.RegisterValidation("hhmm", validateHHMM)
	validate.RegisterStructValidation(validateRouteStatsSpan, routeStatsQuery{})
	validate.RegisterStructValidation(validateRouteOutlookSchedule, routeOutlookQuery{})
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

	filter := store.RouteStatsFilter{
		Origin:       q.Origin,
		Dest:         q.Dest,
		StartDate:    q.StartDate.Format("2006-01-02"),
		EndDate:      q.EndDate.Format("2006-01-02"),
		Carrier:      q.Carrier,
		FlightNumber: q.FlightNumber,
		DaysOfWeek:   uniqueDays(q.DaysOfWeek),
	}
	if err := filter.Validate(); err != nil {
		return store.RouteStatsFilter{}, err
	}

	return filter, nil
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

	window := store.ResolveDepTimeWindowMinutes(q.DepTimeWindowMinutes)

	date := ""
	if !q.Date.IsZero() {
		date = q.Date.Format("2006-01-02")
	}

	dayOfWeek := 0
	if q.DayOfWeek != nil {
		dayOfWeek = *q.DayOfWeek
	}

	day, err := store.ParseOutlookDay(date, dayOfWeek)
	if err != nil {
		return store.RouteOutlookFilter{}, err
	}

	filter := store.RouteOutlookFilter{
		Origin:               q.Origin,
		Dest:                 q.Dest,
		Carrier:              q.Carrier,
		DayOfWeek:            day,
		DepTime:              store.FormatHHMM(q.DepTime),
		DepTimeWindowMinutes: window,
	}
	if err := filter.Validate(); err != nil {
		return store.RouteOutlookFilter{}, err
	}

	return filter, nil
}

func ParseRouteTravelWindows(r *http.Request) (store.RouteTravelWindowsFilter, error) {
	var q routeTravelWindowsQuery
	if err := BindQuery(r, &q); err != nil {
		return store.RouteTravelWindowsFilter{}, err
	}

	normalizeQueryStrings(&q)

	if err := Validate(q); err != nil {
		return store.RouteTravelWindowsFilter{}, err
	}

	return store.RouteTravelWindowsFilter{
		Origin:  q.Origin,
		Dest:    q.Dest,
		Carrier: q.Carrier,
	}, nil
}

func validateRouteStatsSpan(sl validator.StructLevel) {
	q := sl.Current().Interface().(routeStatsQuery)
	reportDateSpan(sl, q.StartDate, q.EndDate)
}

func validateRouteOutlookSchedule(sl validator.StructLevel) {
	q := sl.Current().Interface().(routeOutlookQuery)

	date := ""
	if !q.Date.IsZero() {
		date = q.Date.Format("2006-01-02")
	}

	day := 0
	if q.DayOfWeek != nil {
		day = *q.DayOfWeek
	}

	if _, err := store.ParseOutlookDay(date, day); errors.Is(err, store.ErrOutlookSchedule) {
		sl.ReportError(q.Date, "Date", "date", "outlook_schedule", "")
	}
}

func reportDateSpan(sl validator.StructLevel, start, end time.Time) {
	if !store.ExceedsMaxStatsSpan(start, end) {
		return
	}

	sl.ReportError(end, "EndDate", "end_date", "date_span", strconv.Itoa(store.MaxStatsSpanDays))
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
