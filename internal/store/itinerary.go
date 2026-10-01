package store

import (
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

// LayoverMinutes is the gap from inbound arrival to outbound departure.
// Each instant is that leg's local date plus clock time, so a later outbound
// date is an overnight or multi-day connection. A negative gap is not a layover.
func LayoverMinutes(inboundDate, inboundArr, outboundDate, outboundDep string) (int, bool) {
	in, ok := localDateTime(inboundDate, inboundArr)
	if !ok {
		return 0, false
	}

	out, ok := localDateTime(outboundDate, outboundDep)
	if !ok {
		return 0, false
	}

	mins := int(out.Sub(in).Minutes())
	if mins < 0 {
		return 0, false
	}

	return mins, true
}

// ConnectionType classifies the hop through the connection airport.
// An endpoint is international when its country differs from that airport.
func ConnectionType(originCountry, hubCountry, destCountry string) (string, bool) {
	originCountry = normalizeCountry(originCountry)
	hubCountry = normalizeCountry(hubCountry)
	destCountry = normalizeCountry(destCountry)

	if originCountry == "" || hubCountry == "" || destCountry == "" {
		return "", false
	}

	inboundIntl := originCountry != hubCountry
	outboundIntl := destCountry != hubCountry

	switch {
	case !inboundIntl && !outboundIntl:
		return model.ConnectionDomesticToDomestic, true
	case !inboundIntl && outboundIntl:
		return model.ConnectionDomesticToInternational, true
	case inboundIntl && !outboundIntl:
		return model.ConnectionInternationalToDomestic, true
	default:
		return model.ConnectionInternationalToInternational, true
	}
}

func AssessConnection(inbound, outbound model.ItineraryLeg, outlook *model.RouteOutlook, countries map[string]string) model.ItineraryConnection {
	conn := model.ItineraryConnection{
		Airport:    inbound.Dest,
		Status:     model.ConnectionUnknown,
		Confidence: model.ConfidenceUnknown,
	}

	if mins, ok := LayoverMinutes(inbound.Date, inbound.ArrTime, outbound.Date, outbound.DepTime); ok {
		conn.LayoverMinutes = &mins
	}

	if !sameAirport(inbound.Dest, outbound.Origin) {
		return conn
	}

	originCountry, okO := airportCountry(countries, inbound.Origin)
	hubCountry, okH := airportCountry(countries, inbound.Dest)
	destCountry, okD := airportCountry(countries, outbound.Dest)

	if !okO || !okH || !okD {
		return conn
	}

	connectionType, ok := ConnectionType(originCountry, hubCountry, destCountry)
	if !ok {
		return conn
	}

	conn.ConnectionType = &connectionType

	if outlook == nil {
		return conn
	}

	conn.Confidence = outlook.DecisionConfidence()

	minutes := outlook.Connection.ForType(connectionType)
	if minutes == nil {
		return conn
	}

	recommended := minutes.RecommendedMinutes
	loose := minutes.LooseMinutes
	conn.RecommendedMinutes = &recommended
	conn.LooseMinutes = &loose

	if conn.LayoverMinutes == nil {
		return conn
	}

	slack := *conn.LayoverMinutes - recommended
	conn.SlackMinutes = &slack
	conn.Status = model.ConnectionStatus(*conn.LayoverMinutes, minutes)

	return conn
}

func BuildItineraryOutlook(legs []model.ItineraryLeg, outlooks []*model.RouteOutlook, legErrors []string, countries map[string]string) model.ItineraryOutlookResponse {
	out := model.ItineraryOutlookResponse{
		Legs:        make([]model.ItineraryOutlookLeg, len(legs)),
		Connections: []model.ItineraryConnection{},
	}

	if len(legs) > 1 {
		out.Connections = make([]model.ItineraryConnection, 0, len(legs)-1)
	}

	for i, leg := range legs {
		day, _ := ISOWeekdayFromDate(leg.Date)

		var errMsg *string

		if i < len(legErrors) && legErrors[i] != "" {
			msg := legErrors[i]
			errMsg = &msg
		}

		var outlook *model.RouteOutlook
		if errMsg == nil && i < len(outlooks) {
			outlook = outlooks[i]
		}

		out.Legs[i] = model.ItineraryOutlookLeg{
			Index:                i,
			Origin:               leg.Origin,
			Dest:                 leg.Dest,
			Carrier:              leg.Carrier,
			Date:                 leg.Date,
			DepTime:              FormatHHMM(leg.DepTime),
			ArrTime:              FormatHHMM(leg.ArrTime),
			DepTimeWindowMinutes: ResolveDepTimeWindowMinutes(leg.DepTimeWindowMinutes),
			DayOfWeek:            day,
			Outlook:              outlook,
			Error:                errMsg,
		}
	}

	for i := 0; i < len(legs)-1; i++ {
		var inbound *model.RouteOutlook
		if out.Legs[i].Error == nil {
			inbound = out.Legs[i].Outlook
		}

		conn := AssessConnection(legs[i], legs[i+1], inbound, countries)
		conn.AfterLeg = i
		out.Connections = append(out.Connections, conn)
	}

	return out
}

func localDateTime(date, hhmm string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(date))
	if err != nil {
		return time.Time{}, false
	}

	mins, ok := HHMMToMinutes(hhmm)
	if !ok {
		return time.Time{}, false
	}

	return t.Add(time.Duration(mins) * time.Minute), true
}

func airportCountry(countries map[string]string, code string) (string, bool) {
	if countries == nil {
		return "", false
	}

	country, ok := countries[strings.ToUpper(strings.TrimSpace(code))]
	if !ok {
		return "", false
	}

	country = normalizeCountry(country)
	if country == "" {
		return "", false
	}

	return country, true
}

func normalizeCountry(country string) string {
	return strings.ToUpper(strings.TrimSpace(country))
}

func sameAirport(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
