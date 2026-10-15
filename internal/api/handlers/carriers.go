package handlers

import (
	"errors"
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
//	@Description	On-time stats for a marketing carrier, plus best and worst routes and airports (on-time rate, at least 30 flights, top and bottom 5). Dates are optional and default to the trailing 90 days ending at the carrier's latest flight date (max span 366 days).
//	@Tags			carriers,external
//	@Produce		json
//	@Param			carrier		query		string	true	"Marketing carrier code"	minlength(2)	maxlength(2)
//	@Param			start_date	query		string	false	"Range start (YYYY-MM-DD); required if end_date is set"	Format(date)
//	@Param			end_date	query		string	false	"Range end (YYYY-MM-DD), on or after start_date; required if start_date is set"	Format(date)
//	@Param			state		query		string	false	"Origin or dest state code"	minlength(2)	maxlength(2)
//	@Success		200			{object}	model.CarrierStats
//	@Failure		400			{object}	ErrorResponse
//	@Failure		401			{object}	ErrorResponse
//	@Failure		403			{object}	ErrorResponse
//	@Failure		404			{object}	ErrorResponse
//	@Failure		429			{object}	ErrorResponse
//	@Failure		500			{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/carriers/stats [get]
func (h *CarriersHandler) Stats(w http.ResponseWriter, r *http.Request) {
	filter, err := query.ParseCarrierStats(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	stats, err := h.store.CarrierStats(r.Context(), filter)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, ErrorResponse{Error: "carrier stats not found"})
			return
		}

		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: "failed to compute carrier stats"})

		return
	}

	writeJSON(w, http.StatusOK, stats)
}
