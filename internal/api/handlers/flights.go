package handlers

import (
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/api/query"
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
	filter, err := query.ParseFlightsList(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{jsonErrKey: err.Error()})
		return
	}

	flights, err := h.store.ListOnTimeFlights(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to list flights"})
		return
	}

	if flights == nil {
		flights = []*model.OnTimeFlight{}
	}

	writeJSON(w, http.StatusOK, flights)
}
