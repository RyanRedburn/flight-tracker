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
//	@Description	Aggregated on-time, delay, cancellation, and diversion stats for a route over a date range (max 366 days). Origin and dest are 3-letter airport codes. Days of week use 1=Monday through 7=Sunday. When carrier is omitted, carrier_on_time lists per-marketing-carrier on-time rates and flight counts for the same filters. The field is omitted entirely when carrier is set. Returns 404 when the origin/destination has no flight-performance data, or when carrier is set and that route and carrier have none. Date, weekday, and flight-number filters that match nothing still return 200.
//	@Tags			routes,external
//	@Produce		json
//	@Param			origin			query		string	true	"Origin airport IATA code"	minlength(3)	maxlength(3)
//	@Param			dest			query		string	true	"Destination airport IATA code"	minlength(3)	maxlength(3)
//	@Param			start_date		query		string	true	"Range start (YYYY-MM-DD)"	Format(date)
//	@Param			end_date		query		string	true	"Range end (YYYY-MM-DD), on or after start_date"	Format(date)
//	@Param			carrier			query		string	false	"Marketing carrier code (required if flight_number is set)"	minlength(2)	maxlength(2)
//	@Param			flight_number	query		string	false	"Flight number (requires carrier)"
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
//	@Description	On-time, delay, cancellation, and diversion probabilities for a carrier/route/day/departure-time window, based on the trailing analysis period. Day of week uses 1=Monday through 7=Sunday. dep_time is local departure time as HHmm (e.g. 0700). dep_time_window_minutes defaults to 30 (max 120). Returns 404 when the origin, destination, and carrier have no flight-performance data. A day-of-week or departure-time window that matches nothing still returns 200.
//	@Tags			routes,external
//	@Produce		json
//	@Param			origin						query		string	true	"Origin airport IATA code"	minlength(3)	maxlength(3)
//	@Param			dest						query		string	true	"Destination airport IATA code"	minlength(3)	maxlength(3)
//	@Param			carrier						query		string	true	"Marketing carrier code"	minlength(2)	maxlength(2)
//	@Param			day_of_week					query		int		true	"Day of week (1=Mon … 7=Sun)"	minimum(1)	maximum(7)
//	@Param			dep_time					query		string	true	"Scheduled departure time (HHmm)"
//	@Param			dep_time_window_minutes		query		int		false	"Minutes around dep_time to include (default 30, max 120)"	minimum(1)	maximum(120)
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
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "route outlook not found"})
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
//	@Description	Typical-year on-time rates by month, day of week (1=Mon … 7=Sun), and scheduled local departure hour, pooled from the last up to 2 years of flight-performance data for the origin/destination (and marketing carrier when set). Best/worst lists are the top and bottom 3 eligible buckets. The carrier field is omitted when the carrier query param is not set. Returns 404 when the route (or route+carrier) has no flight-performance data.
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
