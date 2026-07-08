package handlers

import (
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

func (h *RoutesHandler) Stats(w http.ResponseWriter, r *http.Request) {
	filter, err := query.ParseRouteStats(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{jsonErrKey: err.Error()})
		return
	}

	stats, err := h.store.RouteStats(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to compute route stats"})
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *RoutesHandler) Outlook(w http.ResponseWriter, r *http.Request) {
	filter, err := query.ParseRouteOutlook(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{jsonErrKey: err.Error()})
		return
	}

	outlook, err := h.store.RouteOutlook(r.Context(), filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{jsonErrKey: "failed to compute route outlook"})
		return
	}

	writeJSON(w, http.StatusOK, outlook)
}
