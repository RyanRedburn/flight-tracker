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
//	@Description	Historical outlook for each of 2–4 ordered legs, plus connection risk between adjacent legs. Each leg uses the same sample as route outlook, with weekday taken from its date. A valid body always returns 200. A leg whose origin, destination, and carrier have no flight-performance history has a null outlook and an error; that is the route-outlook 404 case, not sample_reason empty_sample. A seen leg embeds the route outlook, including sample_reason: empty_sample (confidence unknown, estimate fields null, insufficient_sample false), insufficient_sample (confidence low, the bool true), or sufficient (confidence high). status is loose when layover_minutes is at least loose_minutes, ok when it meets recommended_minutes but not loose_minutes, tight when shorter, and unknown when the layover or the inbound threshold is missing. Thresholds are missing for empty_sample and insufficient_sample, so status stays unknown; a classified same-airport connection still copies that outlook confidence (unknown or low). recommended_minutes and loose_minutes are copied from the inbound outlook connection bucket for connection_type. confidence matches that outlook for a classified same-airport connection. It stays unknown when the airports differ or the connection type cannot be classified, even if the inbound outlook confidence is high. A negative gap is not a layover, so status stays unknown even when thresholds are present. reason is evaluated for a same-day classification, overnight when the outbound local date is later (including multi-day) and the hop was still classified, multi_airport when the airports differ, missing_country when the domestic/international type cannot be determined, missing_outlook when the inbound leg has no outlook, empty_sample or insufficient_sample from the inbound sample_reason, missing_threshold when a sufficient sample has no bucket for that type, negative_layover when departure is before arrival, and invalid_schedule when the layover cannot be read. overnight is also true for a later outbound date when another reason explains unknown. floor_only is copied from that bucket and is null when no bucket was used.
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
