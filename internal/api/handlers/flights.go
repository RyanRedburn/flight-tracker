package handlers

import (
	"net/http"
	"strconv"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type FlightsHandler struct {
	store store.Store
}

func NewFlightsHandler(s store.Store) *FlightsHandler {
	return &FlightsHandler{store: s}
}

func (h *FlightsHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := store.OnTimeFlightFilter{
		FlightDate: r.URL.Query().Get("flight_date"),
		Origin:     r.URL.Query().Get("origin"),
		Dest:       r.URL.Query().Get("dest"),
		Limit:      50,
	}

	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			writeError(w, http.StatusBadRequest, "invalid limit")
			return
		}

		filter.Limit = parsed
	}

	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			writeError(w, http.StatusBadRequest, "invalid offset")
			return
		}

		filter.Offset = parsed
	}

	if err := filter.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	flights, err := h.store.ListOnTimeFlights(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list flights")
		return
	}

	if flights == nil {
		flights = []*model.OnTimeFlight{}
	}

	writeJSON(w, http.StatusOK, flights)
}
