package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
	"github.com/RyanRedburn/flight-tracker/internal/store"
	"github.com/RyanRedburn/flight-tracker/internal/store/storetest"
)

func TestItineraryOutlookLegNotFoundVsHardError(t *testing.T) {
	const (
		bos  = "BOS"
		ord  = "ORD"
		lax  = "LAX"
		date = "2026-10-06"
	)

	legs := []model.ItineraryLeg{
		{Origin: bos, Dest: ord, Carrier: "UA", Date: date, DepTime: "0700", ArrTime: "0905"},
		{Origin: ord, Dest: lax, Carrier: "UA", Date: date, DepTime: "1100", ArrTime: "1400"},
	}
	countries := func(context.Context, []string) (map[string]string, error) {
		return map[string]string{bos: "US", ord: "US", lax: "US"}, nil
	}

	t.Run("leg not found", func(t *testing.T) {
		stub := &storetest.Stub{
			RouteOutlookFn: func(_ context.Context, filter store.RouteOutlookFilter) (*model.RouteOutlook, error) {
				if filter.Origin == bos {
					return nil, store.ErrNotFound
				}

				return &model.RouteOutlook{
					OutlookResult: model.OutlookResult{
						Analysis: model.Analysis{SampleSize: 12, Confidence: model.ConfidenceHigh},
					},
				}, nil
			},
			ListAirportCountriesByIATAFn: countries,
		}

		got, err := store.ItineraryOutlook(context.Background(), stub, legs)
		if err != nil {
			t.Fatalf("ItineraryOutlook: %v", err)
		}

		if got.Legs[0].Error == nil || *got.Legs[0].Error != store.ItineraryLegOutlookNotFound || got.Legs[0].Outlook != nil {
			t.Fatalf("first leg = %+v", got.Legs[0])
		}

		if got.Legs[1].Error != nil || got.Legs[1].Outlook == nil {
			t.Fatalf("second leg = %+v", got.Legs[1])
		}
	})

	t.Run("hard error", func(t *testing.T) {
		hard := errors.New("outlook failed")
		stub := &storetest.Stub{
			RouteOutlookFn: func(context.Context, store.RouteOutlookFilter) (*model.RouteOutlook, error) {
				return nil, hard
			},
			ListAirportCountriesByIATAFn: countries,
		}

		_, err := store.ItineraryOutlook(context.Background(), stub, legs)
		if !errors.Is(err, hard) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestItineraryLegRouteOutlookFilterWindow(t *testing.T) {
	window := 45

	set := store.ItineraryLegRouteOutlookFilter(model.ItineraryLeg{
		Origin:               "ORD",
		Dest:                 "LAX",
		Carrier:              "UA",
		Date:                 "2026-10-06",
		DepTime:              "0700",
		ArrTime:              "0900",
		DepTimeWindowMinutes: &window,
	})
	if !set.DepTimeWindowSet || set.DepTimeWindowMinutes != window {
		t.Fatalf("explicit window = %+v", set)
	}

	def := store.ItineraryLegRouteOutlookFilter(model.ItineraryLeg{
		Origin:  "ORD",
		Dest:    "LAX",
		Carrier: "UA",
		Date:    "2026-10-06",
		DepTime: "0700",
		ArrTime: "0900",
	})
	if def.DepTimeWindowSet || def.DepTimeWindowMinutes != store.DefaultDepTimeWindowMinutes {
		t.Fatalf("default window = %+v", def)
	}
}
