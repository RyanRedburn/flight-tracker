package store

import (
	"context"
	"errors"
	"sync"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const ItineraryLegOutlookNotFound = "route outlook not found"

// ItineraryOutlook loads per-leg route outlooks and builds the itinerary response.
func ItineraryOutlook(ctx context.Context, s Store, legs []model.ItineraryLeg) (model.ItineraryOutlookResponse, error) {
	outlooks := make([]*model.RouteOutlook, len(legs))
	legErrors := make([]string, len(legs))

	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		hardErr    error
		countries  map[string]string
		countryErr error
	)

	wg.Add(1 + len(legs))

	go func() {
		defer wg.Done()

		codes := make([]string, 0, len(legs)*2)
		for _, leg := range legs {
			codes = append(codes, leg.Origin, leg.Dest)
		}

		countries, countryErr = s.ListAirportCountriesByIATA(ctx, codes)
	}()

	for i, leg := range legs {
		go func(i int, leg model.ItineraryLeg) {
			defer wg.Done()

			outlook, err := s.RouteOutlook(ctx, ItineraryLegRouteOutlookFilter(leg))
			if err != nil {
				if errors.Is(err, ErrNotFound) {
					legErrors[i] = ItineraryLegOutlookNotFound
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

	if countryErr != nil {
		return model.ItineraryOutlookResponse{}, countryErr
	}

	if hardErr != nil {
		return model.ItineraryOutlookResponse{}, hardErr
	}

	return BuildItineraryOutlook(legs, outlooks, legErrors, countries), nil
}

// ItineraryLegRouteOutlookFilter maps one itinerary leg to a route outlook filter.
func ItineraryLegRouteOutlookFilter(leg model.ItineraryLeg) RouteOutlookFilter {
	day, _ := ISOWeekdayFromDate(leg.Date)

	return RouteOutlookFilter{
		Origin:               leg.Origin,
		Dest:                 leg.Dest,
		Carrier:              leg.Carrier,
		DayOfWeek:            day,
		DepTime:              FormatHHMM(leg.DepTime),
		DepTimeWindowMinutes: ResolveDepTimeWindowMinutes(leg.DepTimeWindowMinutes),
		DepTimeWindowSet:     leg.DepTimeWindowMinutes != nil,
	}
}
