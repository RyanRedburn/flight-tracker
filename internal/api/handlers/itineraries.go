package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
)

type ItinerariesHandler struct {
	store store.Store
}

func NewItinerariesHandler(s store.Store) *ItinerariesHandler {
	return &ItinerariesHandler{store: s}
}

// ItineraryOutlook returns historical outlook and connection risk for ordered legs.
//
//	@Summary		Itinerary booking outlook
//	@Description	Historical outlook for 2–4 ordered legs and each connection. Always 200. A leg with no history has a null outlook and an error, not empty_sample.
//	@Tags			itineraries,external
//	@Accept			json
//	@Produce		json
//	@Param			body	body		model.ItineraryOutlookRequest	true	"Ordered legs"
//	@Success		200		{object}	model.ItineraryOutlookResponse
//	@Failure		400		{object}	ErrorResponse
//	@Failure		401		{object}	ErrorResponse
//	@Failure		403		{object}	ErrorResponse
//	@Failure		429		{object}	ErrorResponse
//	@Failure		500		{object}	ErrorResponse
//	@Security		ApiKeyAuth
//	@Router			/api/v1/itineraries/outlook [post]
func (h *ItinerariesHandler) ItineraryOutlook(w http.ResponseWriter, r *http.Request) {
	var req model.ItineraryOutlookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: errInvalidJSONBody})
		return
	}

	if err := req.Validate(); err != nil {
		writeJSON(w, http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}

	out, err := store.ItineraryOutlook(r.Context(), h.store, req.Legs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedItineraryOutlook})
		return
	}

	writeJSON(w, http.StatusOK, out)
}
