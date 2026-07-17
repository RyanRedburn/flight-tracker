package store

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	MinOutlookSampleSize        = 10
	OutlookLookbackDays         = 365
	MaxStatsSpanDays            = 366
	DefaultDepTimeWindowMinutes = 30
	MaxDepTimeWindowMinutes     = 120
)

type RouteStatsFilter struct {
	Origin       string
	Dest         string
	StartDate    string
	EndDate      string
	Carrier      string
	FlightNumber string
	DaysOfWeek   []int
}

type RouteOutlookFilter struct {
	Origin               string
	Dest                 string
	Carrier              string
	DayOfWeek            int
	DepTime              string
	DepTimeWindowMinutes int
}

// FlightPerf is a row slice used for route aggregations.
type FlightPerf struct {
	FlightDate        string
	DayOfWeek         string
	Origin            string
	Dest              string
	Carrier           string
	FlightNumber      string
	CRSDepTime        string
	ArrDelayMinutes   string
	DepDelayMinutes   string
	ArrDel15          string
	DepDel15          string
	Cancelled         string
	CancellationCode  string
	Diverted          string
	CarrierDelay      string
	WeatherDelay      string
	NASDelay          string
	SecurityDelay     string
	LateAircraftDelay string
	DivAirports       [5]string
}

func FlightPerfFromFlightPerformance(f *model.FlightPerformance) FlightPerf {
	return FlightPerf{
		FlightDate:        f.FlightDate,
		DayOfWeek:         f.DayOfWeek,
		Origin:            f.Origin,
		Dest:              f.Dest,
		Carrier:           f.IATA_Code_Marketing_Airline,
		FlightNumber:      f.Flight_Number_Marketing_Airline,
		CRSDepTime:        f.CRSDepTime,
		ArrDelayMinutes:   f.ArrDelayMinutes,
		DepDelayMinutes:   f.DepDelayMinutes,
		ArrDel15:          f.ArrDel15,
		DepDel15:          f.DepDel15,
		Cancelled:         f.Cancelled,
		CancellationCode:  f.CancellationCode,
		Diverted:          f.Diverted,
		CarrierDelay:      f.CarrierDelay,
		WeatherDelay:      f.WeatherDelay,
		NASDelay:          f.NASDelay,
		SecurityDelay:     f.SecurityDelay,
		LateAircraftDelay: f.LateAircraftDelay,
		DivAirports: [5]string{
			f.Div1Airport,
			f.Div2Airport,
			f.Div3Airport,
			f.Div4Airport,
			f.Div5Airport,
		},
	}
}

func AggregateRouteStats(filter RouteStatsFilter, rows []FlightPerf) *model.RouteStats {
	stats := &model.RouteStats{
		Origin:    filter.Origin,
		Dest:      filter.Dest,
		StartDate: filter.StartDate,
		EndDate:   filter.EndDate,
		Filters: model.RouteStatsFilters{
			Carrier:      filter.Carrier,
			FlightNumber: filter.FlightNumber,
			DaysOfWeek:   append([]int(nil), filter.DaysOfWeek...),
		},
		DiversionAirports: []model.AirportCount{},
		CancellationCodes: []model.CancellationCodeCount{},
	}
	if stats.Filters.DaysOfWeek == nil {
		stats.Filters.DaysOfWeek = []int{}
	}

	matched := filterStatsRows(filter, rows)

	stats.Flights = len(matched)
	if stats.Flights == 0 {
		return stats
	}

	var (
		arrDelays         []float64
		arrDelaysDelayed  []float64
		depDelaySum       float64
		depDelayCount     int
		depDelayWhenSum   float64
		depDelayWhenCount int
		causeCarrierSum   float64
		causeWeatherSum   float64
		causeNASSum       float64
		causeSecuritySum  float64
		causeLateSum      float64
		causeCount        int
		divCounts         = map[string]int{}
		cancelCounts      = map[string]int{}
	)

	for _, row := range matched {
		cancelled := isFlagSet(row.Cancelled)
		diverted := isFlagSet(row.Diverted)

		if cancelled {
			stats.Cancelled++

			if code := strings.TrimSpace(row.CancellationCode); code != "" {
				cancelCounts[code]++
			}

			continue
		}

		if diverted {
			stats.Diverted++

			for _, airport := range row.DivAirports {
				if airport = strings.TrimSpace(airport); airport != "" {
					divCounts[airport]++
				}
			}

			continue
		}

		if isFlagSet(row.ArrDel15) {
			stats.Delayed++

			if v, ok := parseFloat(row.ArrDelayMinutes); ok {
				arrDelaysDelayed = append(arrDelaysDelayed, v)
			}

			if v, ok := parseFloat(row.DepDelayMinutes); ok {
				depDelayWhenSum += v
				depDelayWhenCount++
			}

			causeCount++
			causeCarrierSum += parseFloatOrZero(row.CarrierDelay)
			causeWeatherSum += parseFloatOrZero(row.WeatherDelay)
			causeNASSum += parseFloatOrZero(row.NASDelay)
			causeSecuritySum += parseFloatOrZero(row.SecurityDelay)
			causeLateSum += parseFloatOrZero(row.LateAircraftDelay)
		} else {
			stats.OnTime++
		}

		if v, ok := parseFloat(row.ArrDelayMinutes); ok {
			arrDelays = append(arrDelays, v)
		}

		if v, ok := parseFloat(row.DepDelayMinutes); ok {
			depDelaySum += v
			depDelayCount++
		}
	}

	stats.OnTimeRate = rate(stats.OnTime, stats.Flights)
	stats.DelayRate = rate(stats.Delayed, stats.Flights)
	stats.CancellationRate = rate(stats.Cancelled, stats.Flights)
	stats.DiversionRate = rate(stats.Diverted, stats.Flights)
	stats.AvgArrivalDelayMinutes = mean(arrDelays)
	stats.MedianArrivalDelayMinutes = median(arrDelays)
	stats.AvgArrivalDelayWhenDelayed = mean(arrDelaysDelayed)
	stats.MedianArrivalDelayWhenDelayed = median(arrDelaysDelayed)
	stats.AvgDepartureDelayMinutes = safeDiv(depDelaySum, float64(depDelayCount))
	stats.AvgDepartureDelayWhenDelayed = safeDiv(depDelayWhenSum, float64(depDelayWhenCount))

	if causeCount > 0 {
		n := float64(causeCount)
		stats.DelayCausesAvgMinutes = model.DelayCausesAvgMinutes{
			Carrier:      causeCarrierSum / n,
			Weather:      causeWeatherSum / n,
			NAS:          causeNASSum / n,
			Security:     causeSecuritySum / n,
			LateAircraft: causeLateSum / n,
		}
	}

	stats.DiversionAirports = airportCounts(divCounts)
	stats.CancellationCodes = cancellationCodeCounts(cancelCounts)

	return stats
}

func AggregateRouteOutlook(filter RouteOutlookFilter, rows []FlightPerf) *model.RouteOutlook {
	out := &model.RouteOutlook{
		Origin:               filter.Origin,
		Dest:                 filter.Dest,
		Carrier:              filter.Carrier,
		DayOfWeek:            filter.DayOfWeek,
		DepTime:              filter.DepTime,
		DepTimeWindowMinutes: filter.DepTimeWindowMinutes,
	}

	analysisEnd := maxFlightDate(rows, filter.Origin, filter.Dest, filter.Carrier)
	if analysisEnd == "" {
		return out
	}

	endTime, err := time.Parse("2006-01-02", analysisEnd)
	if err != nil {
		return out
	}

	analysisStart := endTime.AddDate(0, 0, -OutlookLookbackDays).Format("2006-01-02")
	out.AnalysisStart = analysisStart
	out.AnalysisEnd = analysisEnd

	matched := filterOutlookRows(filter, rows, analysisStart, analysisEnd)
	out.SampleSize = len(matched)

	out.InsufficientSample = out.SampleSize > 0 && out.SampleSize < MinOutlookSampleSize
	if out.SampleSize == 0 {
		return out
	}

	var (
		onTime, delayed, cancelled, diverted int
		arrDelays                            []float64
		arrDelaysDelayed                     []float64
		depDelaySum                          float64
		depDelayCount                        int
	)

	for _, row := range matched {
		if isFlagSet(row.Cancelled) {
			cancelled++
			continue
		}

		if isFlagSet(row.Diverted) {
			diverted++
			continue
		}

		if isFlagSet(row.ArrDel15) {
			delayed++

			if v, ok := parseFloat(row.ArrDelayMinutes); ok {
				arrDelaysDelayed = append(arrDelaysDelayed, v)
			}
		} else {
			onTime++
		}

		if v, ok := parseFloat(row.ArrDelayMinutes); ok {
			arrDelays = append(arrDelays, v)
		}

		if v, ok := parseFloat(row.DepDelayMinutes); ok {
			depDelaySum += v
			depDelayCount++
		}
	}

	n := out.SampleSize
	out.OnTimeProbability = rate(onTime, n)
	out.DelayProbability = rate(delayed, n)
	out.CancellationProbability = rate(cancelled, n)
	out.DiversionProbability = rate(diverted, n)
	out.LikelyArrivalDelayMinutes = mean(arrDelays)
	out.MedianArrivalDelayMinutes = median(arrDelays)
	out.LikelyArrivalDelayWhenDelayed = mean(arrDelaysDelayed)
	out.MedianArrivalDelayWhenDelayed = median(arrDelaysDelayed)
	out.LikelyDepartureDelayMinutes = safeDiv(depDelaySum, float64(depDelayCount))

	return out
}

func filterStatsRows(filter RouteStatsFilter, rows []FlightPerf) []FlightPerf {
	days := map[int]struct{}{}
	for _, d := range filter.DaysOfWeek {
		days[d] = struct{}{}
	}

	out := make([]FlightPerf, 0, len(rows))
	for _, row := range rows {
		if row.Origin != filter.Origin || row.Dest != filter.Dest {
			continue
		}

		if row.FlightDate < filter.StartDate || row.FlightDate > filter.EndDate {
			continue
		}

		if filter.Carrier != "" && row.Carrier != filter.Carrier {
			continue
		}

		if filter.FlightNumber != "" && row.FlightNumber != filter.FlightNumber {
			continue
		}

		if len(days) > 0 {
			dow, ok := parseInt(row.DayOfWeek)
			if !ok {
				continue
			}

			if _, ok := days[dow]; !ok {
				continue
			}
		}

		out = append(out, row)
	}

	return out
}

func filterOutlookRows(filter RouteOutlookFilter, rows []FlightPerf, start, end string) []FlightPerf {
	targetMin, ok := hhmmToMinutes(filter.DepTime)
	if !ok {
		return nil
	}

	out := make([]FlightPerf, 0)

	for _, row := range rows {
		if row.Origin != filter.Origin || row.Dest != filter.Dest || row.Carrier != filter.Carrier {
			continue
		}

		if row.FlightDate < start || row.FlightDate > end {
			continue
		}

		if normalizeDayOfWeek(row.DayOfWeek) != filter.DayOfWeek {
			continue
		}

		depMin, ok := hhmmToMinutes(row.CRSDepTime)
		if !ok {
			continue
		}

		if circularMinuteDistance(targetMin, depMin) > filter.DepTimeWindowMinutes {
			continue
		}

		out = append(out, row)
	}

	return out
}

func maxFlightDate(rows []FlightPerf, origin, dest, carrier string) string {
	latest := ""

	for _, row := range rows {
		if row.Origin != origin || row.Dest != dest || row.Carrier != carrier {
			continue
		}

		if row.FlightDate > latest {
			latest = row.FlightDate
		}
	}

	return latest
}

func CircularMinuteDistance(a, b int) int {
	return circularMinuteDistance(a, b)
}

func circularMinuteDistance(a, b int) int {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}

	if diff > 1440-diff {
		return 1440 - diff
	}

	return diff
}

func HHMMToMinutes(hhmm string) (int, bool) {
	return hhmmToMinutes(hhmm)
}

func hhmmToMinutes(hhmm string) (int, bool) {
	hhmm = strings.TrimSpace(hhmm)
	if hhmm == "" {
		return 0, false
	}
	// Accept 1–4 digit hhmm (e.g. "559", "0559", "700").
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

func isFlagSet(raw string) bool {
	v, ok := parseFloat(raw)
	return ok && v >= 1
}

func parseFloat(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}

	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}

	return v, true
}

func parseFloatOrZero(raw string) float64 {
	v, _ := parseFloat(raw)
	return v
}

func parseInt(raw string) (int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}

	if v, err := strconv.Atoi(raw); err == nil {
		return v, true
	}

	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}

	return int(f), true
}

func normalizeDayOfWeek(raw string) int {
	v, ok := parseInt(raw)
	if !ok {
		return 0
	}

	return v
}

func rate(part, total int) float64 {
	if total == 0 {
		return 0
	}

	return float64(part) / float64(total)
}

func safeDiv(sum, n float64) float64 {
	if n == 0 {
		return 0
	}

	return sum / n
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum float64
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}

	return (sorted[mid-1] + sorted[mid]) / 2
}

func airportCounts(counts map[string]int) []model.AirportCount {
	out := make([]model.AirportCount, 0, len(counts))
	for airport, count := range counts {
		out = append(out, model.AirportCount{Airport: airport, Count: count})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}

		return out[i].Airport < out[j].Airport
	})

	if out == nil {
		return []model.AirportCount{}
	}

	return out
}

func cancellationCodeCounts(counts map[string]int) []model.CancellationCodeCount {
	out := make([]model.CancellationCodeCount, 0, len(counts))
	for code, count := range counts {
		out = append(out, model.CancellationCodeCount{Code: code, Count: count})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}

		return out[i].Code < out[j].Code
	})

	if out == nil {
		return []model.CancellationCodeCount{}
	}

	return out
}
