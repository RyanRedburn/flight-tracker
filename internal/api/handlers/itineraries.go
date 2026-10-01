package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"sync"

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
//	@Description	Historical outlook for 2–4 ordered legs and each connection. Always 200. A leg with no history has a null outlook and an error, not sample_reason empty_sample.
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

	ctx := r.Context()
	outlooks := make([]*model.RouteOutlook, len(req.Legs))
	legErrors := make([]string, len(req.Legs))

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		hardErr    error
		countries  map[string]string
		countryErr error
	)

	wg.Add(1 + len(req.Legs))

	go func() {
		defer wg.Done()

		countries, countryErr = h.store.ListAirportCountriesByIATA(ctx, itineraryAirportCodes(req.Legs))
	}()

	for i, leg := range req.Legs {
		go func(i int, leg model.ItineraryLeg) {
			defer wg.Done()

			outlook, err := h.store.RouteOutlook(ctx, routeOutlookFilter(leg))
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					legErrors[i] = errRouteOutlookNotFound
					return
				}

				mu.Lock()
				hardErr = err
				mu.Unlock()

				return
			}

			outlooks[i] = outlook
		}(i, leg)
	}

	wg.Wait()

	if countryErr != nil || hardErr != nil {
		writeJSON(w, http.StatusInternalServerError, ErrorResponse{Error: errFailedItineraryOutlook})
		return
	}

	writeJSON(w, http.StatusOK, store.BuildItineraryOutlook(req.Legs, outlooks, legErrors, countries))
}

func routeOutlookFilter(leg model.ItineraryLeg) store.RouteOutlookFilter {
	day, _ := store.ISOWeekdayFromDate(leg.Date)

	return store.RouteOutlookFilter{
		Origin:               leg.Origin,
		Dest:                 leg.Dest,
		Carrier:              leg.Carrier,
		DayOfWeek:            day,
		DepTime:              store.FormatHHMM(leg.DepTime),
		DepTimeWindowMinutes: store.ResolveDepTimeWindowMinutes(leg.DepTimeWindowMinutes),
	}
}

func itineraryAirportCodes(legs []model.ItineraryLeg) []string {
	codes := make([]string, 0, len(legs)*2)
	for _, leg := range legs {
		codes = append(codes, leg.Origin, leg.Dest)
	}

	return codes
}
