package handlers

import (
	"errors"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/api/query"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type RoutesHandler struct {
	store store.Store
}

func NewRoutesHandler(s store.Store) *RoutesHandler {
	return &RoutesHandler{store: s}
}

// Stats returns historical on-time performance for a route.
//
//	@Summary		Route performance stats
//	@Description	Aggregated on-time, delay, cancellation, and diversion stats for a route over a date range (max 366 days). When carrier is omitted, carrier_on_time lists per-marketing-carrier on-time rates and flight counts for the same filters. carrier_comparison is ranked when the highest-rate carrier with at least 100 flights differs at 95% from another, and too_close_to_call when it does not. The carrier list is omitted entirely when carrier is set.
//	@Tags			routes,external
//	@Produce		json
//	@Param			origin			query		string	true	"Origin airport IATA code"	minlength(3)	maxlength(3)
//	@Param			dest			query		string	true	"Destination airport IATA code"	minlength(3)	maxlength(3)
//	@Param			start_date		query		string	true	"Range start (YYYY-MM-DD)"	Format(date)
//	@Param			end_date		query		string	true	"Range end (YYYY-MM-DD), on or after start_date"	Format(date)
//	@Param			carrier			query		string	false	"Marketing carrier code (required if flight_number is set)"	minlength(2)	maxlength(2)
//	@Param			flight_number	query		string	false	"Marketing flight number, 1 to 4 digits; leading zeros are ignored (0123 and 123 match)"
//	@Param			days_of_week	query		[]int	false	"Filter to these weekdays (1=Mon … 7=Sun)"	collectionFormat(multi)	minimum(1)	maximum(7)
//	@Success		200				{object}	model.RouteStats
//	@Failure		400				{object}	ErrorResponse
//	@Failure		401				{object}	ErrorResponse
//	@Failure		403				{object}	ErrorResponse
//	@Failure		404				{object}	ErrorResponse
//	@Failure		429				{object}	ErrorResponse
//	@Failure		500				{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/routes/stats [get]
func (h *RoutesHandler) Stats(w http.ResponseWriter, r *http.Request) {
	filter, err := query.ParseRouteStats(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	stats, err := h.store.RouteStats(r.Context(), filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "route stats not found"})
			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to compute route stats"})

		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// Outlook returns booking-style probabilities for a specific flight pattern.
//
//	@Summary		Route booking outlook
//	@Description	On-time, delay, cancellation, and diversion probabilities for one carrier, route, weekday, and departure window. A date selects that weekday in the same historical sample. A null delay on a non-empty sample means that delay was not observed.
//	@Tags			routes,external
//	@Produce		json
//	@Param			origin						query		string	true	"Origin airport IATA code"	minlength(3)	maxlength(3)
//	@Param			dest						query		string	true	"Destination airport IATA code"	minlength(3)	maxlength(3)
//	@Param			carrier						query		string	true	"Marketing carrier code"	minlength(2)	maxlength(2)
//	@Param			day_of_week					query		int		false	"Weekday (1=Mon … 7=Sun); exactly one of day_of_week or date"	minimum(1)	maximum(7)
//	@Param			date						query		string	false	"Calendar date (YYYY-MM-DD); exactly one of day_of_week or date"	Format(date)
//	@Param			dep_time					query		string	true	"Scheduled departure time (HHmm)"
//	@Param			dep_time_window_minutes		query		int		false	"Minutes around dep_time to include (default 60, max 120)"	minimum(1)	maximum(120)
//	@Success		200							{object}	model.RouteOutlook
//	@Failure		400							{object}	ErrorResponse
//	@Failure		401							{object}	ErrorResponse
//	@Failure		403							{object}	ErrorResponse
//	@Failure		404							{object}	ErrorResponse
//	@Failure		429							{object}	ErrorResponse
//	@Failure		500							{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/routes/outlook [get]
func (h *RoutesHandler) Outlook(w http.ResponseWriter, r *http.Request) {
	filter, err := query.ParseRouteOutlook(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	outlook, err := h.store.RouteOutlook(r.Context(), filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: errRouteOutlookNotFound})
			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to compute route outlook"})

		return
	}

	writeJSON(w, http.StatusOK, outlook)
}

// TravelWindows returns typical-year on-time patterns for a route.
//
//	@Summary		Route travel windows
//	@Description	Typical-year on-time rates by month, weekday, and local departure hour, from up to the last 2 years. Best and worst days and hours are the top and bottom 3 buckets that meet the grain minimum. Months need 50 flights. Their lists are the highest-rate month plus months that do not differ from it at 95%, and the same around the lowest-rate month. Both month lists are omitted when that highest-rate month differs from nobody.
//	@Tags			routes,external
//	@Produce		json
//	@Param			origin	query		string	true	"Origin airport IATA code"	minlength(3)	maxlength(3)
//	@Param			dest	query		string	true	"Destination airport IATA code"	minlength(3)	maxlength(3)
//	@Param			carrier	query		string	false	"Marketing carrier code"	minlength(2)	maxlength(2)
//	@Success		200		{object}	model.RouteTravelWindows
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		404		{object}	ErrorResponse
//	@Failure		429		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/routes/travel-windows [get]
func (h *RoutesHandler) TravelWindows(w http.ResponseWriter, r *http.Request) {
	filter, err := query.ParseRouteTravelWindows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	windows, err := h.store.RouteTravelWindows(r.Context(), filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "route travel windows not found"})
			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to compute route travel windows"})

		return
	}

	writeJSON(w, http.StatusOK, windows)
}

// WeatherStats returns on-time performance by METAR category at each end of a route.
//
//	@Summary		Route weather category stats
//	@Description	On-time performance by observed weather category at each end of a route (max 366 days).
//	@Tags			routes,external
//	@Produce		json
//	@Param			origin			query		string	true	"Origin airport IATA code"	minlength(3)	maxlength(3)
//	@Param			dest			query		string	true	"Destination airport IATA code"	minlength(3)	maxlength(3)
//	@Param			start_date		query		string	true	"Range start (YYYY-MM-DD)"	Format(date)
//	@Param			end_date		query		string	true	"Range end (YYYY-MM-DD), on or after start_date"	Format(date)
//	@Param			carrier			query		string	false	"Marketing carrier code (required if flight_number is set)"	minlength(2)	maxlength(2)
//	@Param			flight_number	query		string	false	"Marketing flight number, 1 to 4 digits; leading zeros are ignored (0123 and 123 match)"
//	@Param			days_of_week	query		[]int	false	"Filter to these weekdays (1=Mon … 7=Sun)"	collectionFormat(multi)	minimum(1)	maximum(7)
//	@Success		200				{object}	model.RouteWeatherStats
//	@Failure		400				{object}	ErrorResponse
//	@Failure		401				{object}	ErrorResponse
//	@Failure		403				{object}	ErrorResponse
//	@Failure		404				{object}	ErrorResponse
//	@Failure		429				{object}	ErrorResponse
//	@Failure		500				{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/routes/weather-stats [get]
func (h *RoutesHandler) WeatherStats(w http.ResponseWriter, r *http.Request) {
	filter, err := query.ParseRouteStats(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	stats, err := h.store.RouteWeatherStats(r.Context(), filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "route weather stats not found"})
			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to compute route weather stats"})

		return
	}

	writeJSON(w, http.StatusOK, stats)
}
