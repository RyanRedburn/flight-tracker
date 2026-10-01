package model

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
)

const (
	ConnectionDomesticToDomestic           = "domestic_to_domestic"
	ConnectionDomesticToInternational      = "domestic_to_international"
	ConnectionInternationalToDomestic      = "international_to_domestic"
	ConnectionInternationalToInternational = "international_to_international"
)

var itineraryValidate = validator.New()

func init() {
	itineraryValidate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name, _, _ := strings.Cut(fld.Tag.Get("json"), ",")
		if name != "" && name != "-" {
			return name
		}

		return strings.ToLower(fld.Name)
	})

	_ = itineraryValidate.RegisterValidation("hhmm", validateItineraryHHMM)
}

type ItineraryOutlookRequest struct {
	Legs []ItineraryLeg `json:"legs" validate:"required,min=2,max=4,dive"`
}

type ItineraryLeg struct {
	Origin               string `json:"origin" validate:"required,len=3"`
	Dest                 string `json:"dest" validate:"required,len=3"`
	Carrier              string `json:"carrier" validate:"required,len=2"`
	Date                 string `json:"date" validate:"required,datetime=2006-01-02"`
	DepTime              string `json:"dep_time" validate:"required,hhmm"`
	ArrTime              string `json:"arr_time" validate:"required,hhmm"`
	DepTimeWindowMinutes *int   `json:"dep_time_window_minutes,omitempty" validate:"omitempty,gte=1,lte=120"`
}

type ItineraryOutlookResponse struct {
	Legs        []ItineraryOutlookLeg `json:"legs"`
	Connections []ItineraryConnection `json:"connections"`
}

type ItineraryOutlookLeg struct {
	Index                int           `json:"index"`
	Origin               string        `json:"origin"`
	Dest                 string        `json:"dest"`
	Carrier              string        `json:"carrier"`
	Date                 string        `json:"date"`
	DepTime              string        `json:"dep_time"`
	ArrTime              string        `json:"arr_time"`
	DepTimeWindowMinutes int           `json:"dep_time_window_minutes"`
	DayOfWeek            int           `json:"day_of_week"`
	Outlook              *RouteOutlook `json:"outlook"`
	Error                *string       `json:"error"`
}

type ItineraryConnection struct {
	AfterLeg           int     `json:"after_leg"`
	Airport            string  `json:"airport"`
	LayoverMinutes     *int    `json:"layover_minutes"`
	ConnectionType     *string `json:"connection_type" enums:"domestic_to_domestic,domestic_to_international,international_to_domestic,international_to_international"`
	RecommendedMinutes *int    `json:"recommended_minutes"`
	LooseMinutes       *int    `json:"loose_minutes"`
	SlackMinutes       *int    `json:"slack_minutes"`
	Status             string  `json:"status" enums:"loose,ok,tight,unknown"`
	Confidence         string  `json:"confidence" enums:"high,low,unknown"`
	// Reason explains status. evaluated is a same-day classification.
	// overnight is a classified connection whose outbound local date is later, including multi-day.
	// multi_airport means the airports differ. missing_country means the domestic/international type cannot be determined.
	// missing_outlook means the inbound leg has no outlook, which is the route-outlook 404 case and not empty_sample.
	// empty_sample and insufficient_sample are the inbound sample_reason, so status stays unknown and confidence stays unknown or low.
	// missing_threshold means a sufficient sample has no bucket for this type.
	// negative_layover means departure is before arrival. invalid_schedule means the layover date or clock could not be read.
	Reason string `json:"reason" enums:"evaluated,overnight,multi_airport,missing_country,missing_outlook,empty_sample,insufficient_sample,missing_threshold,negative_layover,invalid_schedule"`
	// Overnight is true when a positive layover uses a later outbound local date.
	// It stays true when reason names a different unknown cause.
	Overnight bool `json:"overnight"`
	// FloorOnly is copied from the inbound outlook bucket. True means that bucket used the static MCT floor.
	// False means the airport minute was present. Null means no bucket was used.
	FloorOnly *bool `json:"floor_only"`
}

func (r *ItineraryOutlookRequest) Validate() error {
	if r == nil {
		return errors.New("invalid json body")
	}

	normalizeItinerary(r)

	if err := itineraryValidate.Struct(r); err != nil {
		return formatItineraryValidationError(err)
	}

	return nil
}

func normalizeItinerary(r *ItineraryOutlookRequest) {
	for i := range r.Legs {
		leg := &r.Legs[i]
		leg.Origin = strings.ToUpper(strings.TrimSpace(leg.Origin))
		leg.Dest = strings.ToUpper(strings.TrimSpace(leg.Dest))
		leg.Carrier = strings.ToUpper(strings.TrimSpace(leg.Carrier))
		leg.Date = strings.TrimSpace(leg.Date)
		leg.DepTime = strings.TrimSpace(leg.DepTime)
		leg.ArrTime = strings.TrimSpace(leg.ArrTime)
	}
}

func validateItineraryHHMM(fl validator.FieldLevel) bool {
	_, ok := itineraryHHMMMinutes(fl.Field().String())
	return ok
}

func itineraryHHMMMinutes(hhmm string) (int, bool) {
	hhmm = strings.TrimSpace(hhmm)
	if hhmm == "" {
		return 0, false
	}

	n, err := strconv.Atoi(hhmm)
	if err != nil || n < 0 || n > 2359 {
		return 0, false
	}

	h := n / 100

	m := n % 100
	if h > 23 || m > 59 {
		return 0, false
	}

	return h*60 + m, true
}

func formatItineraryValidationError(err error) error {
	var validationErrs validator.ValidationErrors
	if !errors.As(err, &validationErrs) {
		return err
	}

	messages := make([]string, 0, len(validationErrs))
	for _, fieldErr := range validationErrs {
		messages = append(messages, formatItineraryFieldError(fieldErr))
	}

	return fmt.Errorf("%s", strings.Join(messages, "; "))
}

func formatItineraryFieldError(fieldErr validator.FieldError) string {
	field := itineraryField(fieldErr)

	switch fieldErr.Tag() {
	case "len":
		return field + " must be exactly " + fieldErr.Param() + " characters"
	case "datetime":
		return field + " must be a valid date (YYYY-MM-DD)"
	case "gte":
		return field + " must be >= " + fieldErr.Param()
	case "lte":
		return field + " must be <= " + fieldErr.Param()
	case "min":
		if fieldErr.Kind() == reflect.Slice || fieldErr.Kind() == reflect.Array {
			return field + " must contain at least " + fieldErr.Param() + " items"
		}

		return field + " must be at least " + fieldErr.Param() + " characters"
	case "max":
		if fieldErr.Kind() == reflect.Slice || fieldErr.Kind() == reflect.Array {
			return field + " must contain at most " + fieldErr.Param() + " items"
		}

		return field + " must be at most " + fieldErr.Param() + " characters"
	case "required":
		return field + " is required"
	case "hhmm":
		return field + " must be a valid local time (hhmm)"
	default:
		return field + " is invalid"
	}
}

func itineraryField(fieldErr validator.FieldError) string {
	ns := fieldErr.Namespace()
	if i := strings.IndexByte(ns, '.'); i >= 0 && i+1 < len(ns) {
		return ns[i+1:]
	}

	return fieldErr.Field()
}
