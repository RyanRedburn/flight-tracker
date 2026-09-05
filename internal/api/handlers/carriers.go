package handlers

import (
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/api/query"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type CarriersHandler struct {
	store store.Store
}

func NewCarriersHandler(s store.Store) *CarriersHandler {
	return &CarriersHandler{store: s}
}

// Stats returns historical on-time performance for a marketing carrier.
//
//	@Summary		Carrier performance stats
//	@Description	Aggregated on-time, delay, cancellation, and diversion stats for a marketing carrier, plus best and worst routes and airports (on-time rate, min 30 flights, top/bottom 5). Carrier is a 2-letter marketing IATA code. Dates are optional together and default to the trailing 90 days ending at the carrier's latest flight date (max span 366 days). State is a 2-letter code matching origin or dest.
//	@Tags			carriers,external
//	@Produce		json
//	@Param			carrier		query		string	true	"Marketing carrier code"	minlength(2)	maxlength(2)
//	@Param			start_date	query		string	false	"Range start (YYYY-MM-DD); required if end_date is set"	Format(date)
//	@Param			end_date	query		string	false	"Range end (YYYY-MM-DD), on or after start_date; required if start_date is set"	Format(date)
//	@Param			state		query		string	false	"Origin or dest state code"	minlength(2)	maxlength(2)
//	@Success		200			{object}	model.CarrierStats
//	@Failure		400			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Router			/api/v1/carriers/stats [get]
func (h *CarriersHandler) Stats(w http.ResponseWriter, r *http.Request) {
	filter, err := query.ParseCarrierStats(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	stats, err := h.store.CarrierStats(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to compute carrier stats"})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}
