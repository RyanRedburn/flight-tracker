package store

import (
	"math"
	"reflect"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

// Single-pass aggregation must match the previous separate scans: overall
// counts and percentile populations, then route and airport groupings. Ranking
// and rounding stay the same, so the carrier stats response stays the same.
func TestCarrierStatsSinglePassMatchesSeparatePasses(t *testing.T) {
	sets := []struct {
		name    string
		flights []carrierFlight
		state   string
	}{
		{name: "tiny", flights: tinyCarrierFlights(), state: ""},
		{name: "tiny IL", flights: tinyCarrierFlights(), state: "IL"},
		{name: "causes", flights: causeBucketFlights(), state: ""},
		{name: "generated", flights: generatedCarrierFlights(), state: ""},
		{name: "generated IL", flights: generatedCarrierFlights(), state: "IL"},
		{name: "cancelled only", flights: []carrierFlight{cancelledOnlyFlight()}, state: ""},
		{name: "one arrival", flights: []carrierFlight{singleArrivalFlight()}, state: ""},
	}

	for _, tt := range sets {
		t.Run(tt.name, func(t *testing.T) {
			separate := aggregateCarrierSeparate(tt.flights, tt.state)
			once := aggregateCarrierOnce(tt.flights, tt.state)

			if !reflect.DeepEqual(separate, once) {
				t.Fatalf("single pass != separate passes\nseparate=%+v\nonce=%+v", separate, once)
			}

			if !reflect.DeepEqual(separate.ranked(), once.ranked()) {
				t.Fatalf("ranked response != \nseparate=%+v\nonce=%+v", separate.ranked(), once.ranked())
			}
		})
	}
}

func TestCarrierStatsTinyPopulations(t *testing.T) {
	flights := tinyCarrierFlights()

	for _, agg := range []carrierPassResult{aggregateCarrierSeparate(flights, ""), aggregateCarrierOnce(flights, "")} {
		if agg.flights != 7 || agg.onTime != 2 || agg.delayed != 3 || agg.cancelled != 1 || agg.diverted != 1 {
			t.Fatalf("counts = %+v", agg.outcomeCounts)
		}

		if agg.medianArr != 10.5 || agg.medianArr != percentileCont(0.5, []float64{-5, 1, 20, 40}) {
			t.Fatalf("median arrival = %v, want 10.5", agg.medianArr)
		}

		if agg.medianArrDelayed != 30 || agg.medianArrDelayed != percentileCont(0.5, []float64{20, 40}) {
			t.Fatalf("median arrival when delayed = %v, want 30", agg.medianArrDelayed)
		}

		if agg.medianDep != 4.5 || agg.medianDep != percentileCont(0.5, []float64{-3, 1, 8, 15}) {
			t.Fatalf("median departure = %v, want 4.5", agg.medianDep)
		}

		if agg.medianDepDelayed != 11.5 || agg.medianDepDelayed != percentileCont(0.5, []float64{8, 15}) {
			t.Fatalf("median departure when delayed = %v, want 11.5", agg.medianDepDelayed)
		}

		if agg.avgArr != 14 || agg.avgArrDelayed != 30 || agg.avgDep != 5.25 || agg.avgDepDelayed != 11.5 {
			t.Fatalf("avgs arr=%v arrDelayed=%v dep=%v depDelayed=%v", agg.avgArr, agg.avgArrDelayed, agg.avgDep, agg.avgDepDelayed)
		}

		if agg.causeCarrier != 10 || agg.causeLate != 10.0/3.0 || agg.causeWeather != 0 || agg.causeNAS != 0 || agg.causeSecurity != 0 {
			t.Fatalf("causes = carrier %v late %v weather %v nas %v security %v", agg.causeCarrier, agg.causeLate, agg.causeWeather, agg.causeNAS, agg.causeSecurity)
		}

		if agg.shareCarrier != 1 || agg.shareLate != 1 || agg.shareUnattributed != 1 || agg.shareWeather != 0 || agg.shareNAS != 0 || agg.shareSecurity != 0 {
			t.Fatalf("shares = %+v", agg)
		}

		assertOutcome(t, agg.route(testTravelOrigin, testAirportLAX), outcomeCounts{flights: 4, onTime: 1, delayed: 2, cancelled: 1})
		assertOutcome(t, agg.route(testAirportSFO, testTravelOrigin), outcomeCounts{flights: 1, diverted: 1})
		assertOutcome(t, agg.route(testAirportMDW, testTravelOrigin), outcomeCounts{flights: 1, delayed: 1})
		assertOutcome(t, agg.route(testAirportLAX, testAirportSFO), outcomeCounts{flights: 1, onTime: 1})
		assertOutcome(t, agg.airport(testTravelOrigin), outcomeCounts{flights: 6, onTime: 1, delayed: 3, cancelled: 1, diverted: 1})
		assertOutcome(t, agg.airport(testAirportLAX), outcomeCounts{flights: 5, onTime: 2, delayed: 2, cancelled: 1})
		assertOutcome(t, agg.airport(testAirportSFO), outcomeCounts{flights: 2, onTime: 1, diverted: 1})
		assertOutcome(t, agg.airport(testAirportMDW), outcomeCounts{flights: 1, delayed: 1})
	}

	for _, agg := range []carrierPassResult{aggregateCarrierSeparate(flights, "IL"), aggregateCarrierOnce(flights, "IL")} {
		if agg.flights != 6 || agg.onTime != 1 || agg.delayed != 3 || agg.cancelled != 1 || agg.diverted != 1 {
			t.Fatalf("IL counts = %+v", agg.outcomeCounts)
		}

		if agg.medianArr != 20 || agg.avgArr != 55.0/3.0 || agg.medianDep != 8 || agg.avgDep != 20.0/3.0 {
			t.Fatalf("IL delays medianArr=%v avgArr=%v medianDep=%v avgDep=%v", agg.medianArr, agg.avgArr, agg.medianDep, agg.avgDep)
		}

		if _, ok := agg.routes[routeKey(testAirportLAX, testAirportSFO)]; ok {
			t.Fatal("LAX-SFO does not touch IL")
		}

		assertOutcome(t, agg.route(testTravelOrigin, testAirportLAX), outcomeCounts{flights: 4, onTime: 1, delayed: 2, cancelled: 1})
		assertOutcome(t, agg.route(testAirportSFO, testTravelOrigin), outcomeCounts{flights: 1, diverted: 1})
		assertOutcome(t, agg.airport(testTravelOrigin), outcomeCounts{flights: 6, onTime: 1, delayed: 3, cancelled: 1, diverted: 1})
		assertOutcome(t, agg.airport(testAirportMDW), outcomeCounts{flights: 1, delayed: 1})

		if _, ok := agg.airports[testAirportLAX]; ok {
			t.Fatal("LAX is not an IL endpoint")
		}

		if _, ok := agg.airports[testAirportSFO]; ok {
			t.Fatal("SFO is not an IL endpoint")
		}
	}
}

func TestCarrierStatsCauseBuckets(t *testing.T) {
	flights := causeBucketFlights()

	for _, agg := range []carrierPassResult{aggregateCarrierSeparate(flights, ""), aggregateCarrierOnce(flights, "")} {
		if agg.delayed != 7 || agg.flights != 7 {
			t.Fatalf("counts = %+v", agg.outcomeCounts)
		}

		if agg.shareCarrier != 1 || agg.shareWeather != 1 || agg.shareNAS != 1 || agg.shareSecurity != 1 || agg.shareLate != 2 || agg.shareUnattributed != 1 {
			t.Fatalf("shares carrier=%d weather=%d nas=%d security=%d late=%d unattributed=%d", agg.shareCarrier, agg.shareWeather, agg.shareNAS, agg.shareSecurity, agg.shareLate, agg.shareUnattributed)
		}

		if agg.causeCarrier != 12.0/7.0 || agg.causeWeather != 4.0/7.0 || agg.causeNAS != 3.0/7.0 || agg.causeSecurity != 2.0/7.0 || agg.causeLate != 16.0/7.0 {
			t.Fatalf("cause avgs carrier=%v weather=%v nas=%v security=%v late=%v", agg.causeCarrier, agg.causeWeather, agg.causeNAS, agg.causeSecurity, agg.causeLate)
		}
	}
}

func TestCarrierStatsGeneratedResponse(t *testing.T) {
	flights := generatedCarrierFlights()

	for _, agg := range []carrierPassResult{aggregateCarrierSeparate(flights, ""), aggregateCarrierOnce(flights, "")} {
		if agg.flights != 373 || agg.onTime != 155 || agg.delayed != 133 || agg.cancelled != 43 || agg.diverted != 42 {
			t.Fatalf("counts = %+v", agg.outcomeCounts)
		}

		stats := agg.ranked()
		if len(stats.BestRoutes) != CarrierStatsTopN || len(stats.WorstRoutes) != CarrierStatsTopN {
			t.Fatalf("routes best=%d worst=%d", len(stats.BestRoutes), len(stats.WorstRoutes))
		}

		if len(stats.BestAirports) != CarrierStatsTopN || len(stats.WorstAirports) != CarrierStatsTopN {
			t.Fatalf("airports best=%d worst=%d", len(stats.BestAirports), len(stats.WorstAirports))
		}

		if _, ok := agg.routes[routeKey(testAirportQQQ, testAirportRRR)]; !ok || agg.route(testAirportQQQ, testAirportRRR).flights != 29 {
			t.Fatal("below-min route should still be grouped")
		}

		if len(stats.BestRoutes) > 0 && (stats.BestRoutes[0].Origin == testAirportQQQ || stats.WorstRoutes[0].Origin == testAirportQQQ) {
			t.Fatal("29-flight route should not be ranked")
		}

		assertGroupedCounts(t, flights, "", agg)
	}

	for _, agg := range []carrierPassResult{aggregateCarrierSeparate(flights, "IL"), aggregateCarrierOnce(flights, "IL")} {
		if agg.flights != 70 || agg.onTime != 47 || agg.delayed != 14 || agg.cancelled != 5 || agg.diverted != 4 {
			t.Fatalf("IL counts = %+v", agg.outcomeCounts)
		}

		stats := agg.ranked()
		if len(stats.BestRoutes) != 2 || len(stats.WorstRoutes) != 0 {
			t.Fatalf("IL routes best=%d worst=%d", len(stats.BestRoutes), len(stats.WorstRoutes))
		}

		if stats.BestRoutes[0].Origin != testAirportAAA || stats.BestRoutes[0].Dest != testAirportBBB || stats.BestRoutes[0].Flights != 40 || stats.BestRoutes[0].OnTime != 32 {
			t.Fatalf("IL best route = %+v", stats.BestRoutes[0])
		}

		if stats.BestRoutes[1].Origin != "UUU" || stats.BestRoutes[1].Dest != testAirportAAA || stats.BestRoutes[1].Flights != 30 || stats.BestRoutes[1].OnTime != 15 {
			t.Fatalf("IL second route = %+v", stats.BestRoutes[1])
		}

		if len(stats.BestAirports) != 1 || stats.BestAirports[0].Airport != testAirportAAA || stats.BestAirports[0].Flights != 70 || len(stats.WorstAirports) != 0 {
			t.Fatalf("IL airports = %+v worst=%+v", stats.BestAirports, stats.WorstAirports)
		}

		if stats.OnTimeRate != math.Round(float64(47)/float64(70)*100)/100 || stats.Analysis.SampleSize != 70 {
			t.Fatalf("IL response on_time_rate=%v sample=%d", stats.OnTimeRate, stats.Analysis.SampleSize)
		}

		assertGroupedCounts(t, flights, "IL", agg)
	}

	separate := aggregateCarrierSeparate(flights, "IL").ranked()
	once := aggregateCarrierOnce(flights, "IL").ranked()

	if !reflect.DeepEqual(separate, once) {
		t.Fatalf("IL response mismatch\nseparate=%+v\nonce=%+v", separate, once)
	}
}

func TestCarrierStatsEmptyAndSingleDelayPopulation(t *testing.T) {
	cancelled := []carrierFlight{cancelledOnlyFlight()}

	for _, agg := range []carrierPassResult{aggregateCarrierSeparate(cancelled, ""), aggregateCarrierOnce(cancelled, "")} {
		if agg.flights != 1 || agg.cancelled != 1 || agg.onTime != 0 || agg.delayed != 0 || agg.diverted != 0 {
			t.Fatalf("counts = %+v", agg.outcomeCounts)
		}

		if agg.medianArr != 0 || agg.avgArr != 0 || agg.medianDep != 0 || agg.avgDep != 0 || agg.causeCarrier != 0 || agg.shareLate != 0 {
			t.Fatalf("empty delay population = %+v", agg)
		}

		stats := agg.ranked()
		if stats.MedianArrivalDelayMinutes != 0 || stats.DelayCausesAvgMinutes != (model.DelayCausesAvgMinutes{}) || len(stats.BestRoutes) != 0 {
			t.Fatalf("response = %+v", stats)
		}
	}

	one := []carrierFlight{singleArrivalFlight()}

	for _, agg := range []carrierPassResult{aggregateCarrierSeparate(one, ""), aggregateCarrierOnce(one, "")} {
		if agg.flights != 1 || agg.onTime != 1 || agg.medianArr != 4 || agg.medianDep != 7 || agg.avgArr != 4 || agg.avgDep != 7 {
			t.Fatalf("single flight = %+v", agg)
		}

		if agg.medianArrDelayed != 0 || agg.avgArrDelayed != 0 {
			t.Fatalf("no delayed population = median %v avg %v", agg.medianArrDelayed, agg.avgArrDelayed)
		}
	}
}

func assertOutcome(t *testing.T, got, want outcomeCounts) {
	t.Helper()

	if got != want {
		t.Fatalf("outcome = %+v, want %+v", got, want)
	}
}

func assertGroupedCounts(t *testing.T, flights []carrierFlight, state string, agg carrierPassResult) {
	t.Helper()

	routes := map[string]int{}
	airports := map[string]int{}

	for _, flight := range flights {
		if !carrierStateMatch(flight, state) {
			continue
		}

		routes[routeKey(flight.origin, flight.dest)]++

		if state == "" || flight.originState == state {
			airports[flight.origin]++
		}

		if state == "" || flight.destState == state {
			airports[flight.dest]++
		}
	}

	if len(routes) != len(agg.routes) {
		t.Fatalf("routes = %d, want %d", len(agg.routes), len(routes))
	}

	for key, want := range routes {
		got, ok := agg.routes[key]
		if !ok || got.flights != want {
			t.Fatalf("route %s flights = %d, want %d", key, got.flights, want)
		}
	}

	if len(airports) != len(agg.airports) {
		t.Fatalf("airports = %d, want %d", len(agg.airports), len(airports))
	}

	for code, want := range airports {
		got, ok := agg.airports[code]
		if !ok || got.flights != want {
			t.Fatalf("airport %s flights = %d, want %d", code, got.flights, want)
		}
	}
}

type carrierFlight struct {
	origin      string
	dest        string
	originState string
	destState   string
	cancelled   float64
	diverted    float64
	arrDel15    float64
	arr         *float64
	dep         *float64
	carrier     *float64
	weather     *float64
	nas         *float64
	security    *float64
	late        *float64
}

type outcomeCounts struct {
	flights   int
	onTime    int
	delayed   int
	cancelled int
	diverted  int
}

type routeCounts struct {
	origin string
	dest   string
	outcomeCounts
}

type carrierPassResult struct {
	outcomeCounts
	avgArr            float64
	medianArr         float64
	avgArrDelayed     float64
	medianArrDelayed  float64
	avgDep            float64
	medianDep         float64
	avgDepDelayed     float64
	medianDepDelayed  float64
	causeCarrier      float64
	causeWeather      float64
	causeNAS          float64
	causeSecurity     float64
	causeLate         float64
	shareCarrier      int
	shareWeather      int
	shareNAS          int
	shareSecurity     int
	shareLate         int
	shareUnattributed int
	routes            map[string]routeCounts
	airports          map[string]outcomeCounts
}

func (r carrierPassResult) route(origin, dest string) outcomeCounts {
	return r.routes[routeKey(origin, dest)].outcomeCounts
}

func (r carrierPassResult) airport(code string) outcomeCounts {
	return r.airports[code]
}

func (r carrierPassResult) ranked() model.CarrierStats {
	routes := make([]model.CarrierRouteStat, 0)

	for _, row := range r.routes {
		if row.flights < CarrierStatsMinSampleSize {
			continue
		}

		routes = append(routes, model.CarrierRouteStat{
			Origin:           row.origin,
			Dest:             row.dest,
			Flights:          row.flights,
			OnTime:           row.onTime,
			Delayed:          row.delayed,
			Cancelled:        row.cancelled,
			Diverted:         row.diverted,
			OnTimeRate:       rate(row.onTime, row.flights),
			DelayRate:        rate(row.delayed, row.flights),
			CancellationRate: rate(row.cancelled, row.flights),
			DiversionRate:    rate(row.diverted, row.flights),
		})
	}

	airports := make([]model.CarrierAirportStat, 0)

	for code, row := range r.airports {
		if row.flights < CarrierStatsMinSampleSize {
			continue
		}

		airports = append(airports, model.CarrierAirportStat{
			Airport:          code,
			Flights:          row.flights,
			OnTime:           row.onTime,
			Delayed:          row.delayed,
			Cancelled:        row.cancelled,
			Diverted:         row.diverted,
			OnTimeRate:       rate(row.onTime, row.flights),
			DelayRate:        rate(row.delayed, row.flights),
			CancellationRate: rate(row.cancelled, row.flights),
			DiversionRate:    rate(row.diverted, row.flights),
		})
	}

	stats := model.CarrierStats{
		Analysis:                        model.Analysis{SampleSize: r.flights},
		Flights:                         r.flights,
		OnTime:                          r.onTime,
		Delayed:                         r.delayed,
		Cancelled:                       r.cancelled,
		Diverted:                        r.diverted,
		OnTimeRate:                      rate(r.onTime, r.flights),
		DelayRate:                       rate(r.delayed, r.flights),
		CancellationRate:                rate(r.cancelled, r.flights),
		DiversionRate:                   rate(r.diverted, r.flights),
		AvgArrivalDelayMinutes:          r.avgArr,
		MedianArrivalDelayMinutes:       r.medianArr,
		AvgArrivalDelayWhenDelayed:      r.avgArrDelayed,
		MedianArrivalDelayWhenDelayed:   r.medianArrDelayed,
		AvgDepartureDelayMinutes:        r.avgDep,
		MedianDepartureDelayMinutes:     r.medianDep,
		AvgDepartureDelayWhenDelayed:    r.avgDepDelayed,
		MedianDepartureDelayWhenDelayed: r.medianDepDelayed,
	}

	if r.delayed > 0 {
		stats.DelayCausesAvgMinutes = model.DelayCausesAvgMinutes{
			Carrier:      r.causeCarrier,
			Weather:      r.causeWeather,
			NAS:          r.causeNAS,
			Security:     r.causeSecurity,
			LateAircraft: r.causeLate,
		}
		stats.DelayCausesShare = DelayCausesShareFromCounts(
			r.delayed,
			r.shareCarrier,
			r.shareWeather,
			r.shareNAS,
			r.shareSecurity,
			r.shareLate,
			r.shareUnattributed,
		)
	}

	stats.BestRoutes, stats.WorstRoutes = RankCarrierRoutes(routes)
	stats.BestAirports, stats.WorstAirports = RankCarrierAirports(airports)
	stats.RoundForResponse()

	return stats
}

func aggregateCarrierOnce(flights []carrierFlight, state string) carrierPassResult {
	var result carrierPassResult

	result.routes = map[string]routeCounts{}
	result.airports = map[string]outcomeCounts{}

	var (
		arrSum, arrDelayedSum, depSum, depDelayedSum        float64
		arrN, arrDelayedN, depN, depDelayedN                int
		arrVals, arrDelayedVals, depVals, depDelayedVals    []float64
		causeCarrier, causeWeather, causeNAS, causeSecurity float64
		causeLate                                           float64
	)

	for _, flight := range flights {
		if !carrierStateMatch(flight, state) {
			continue
		}

		isCancelled, isDiverted, isDelayed := classifyCarrierFlight(flight)
		addOutcome(&result.outcomeCounts, isCancelled, isDiverted, isDelayed)
		addDelay(&arrSum, &arrN, &arrVals, !isCancelled && !isDiverted, flight.arr)
		addDelay(&arrDelayedSum, &arrDelayedN, &arrDelayedVals, isDelayed, flight.arr)
		addDelay(&depSum, &depN, &depVals, !isCancelled && !isDiverted, flight.dep)
		addDelay(&depDelayedSum, &depDelayedN, &depDelayedVals, isDelayed, flight.dep)

		if isDelayed {
			causeCarrier += coalesceDelay(flight.carrier)
			causeWeather += coalesceDelay(flight.weather)
			causeNAS += coalesceDelay(flight.nas)
			causeSecurity += coalesceDelay(flight.security)
			causeLate += coalesceDelay(flight.late)
			addCauseShare(&result, primaryCause(flight))
		}

		addRouteCount(result.routes, flight, isCancelled, isDiverted, isDelayed)

		if state == "" || flight.originState == state {
			addAirportCount(result.airports, flight.origin, isCancelled, isDiverted, isDelayed)
		}

		if state == "" || flight.destState == state {
			addAirportCount(result.airports, flight.dest, isCancelled, isDiverted, isDelayed)
		}
	}

	result.avgArr = mean(arrSum, arrN)
	result.avgArrDelayed = mean(arrDelayedSum, arrDelayedN)
	result.avgDep = mean(depSum, depN)
	result.avgDepDelayed = mean(depDelayedSum, depDelayedN)
	result.medianArr = percentileCont(0.5, arrVals)
	result.medianArrDelayed = percentileCont(0.5, arrDelayedVals)
	result.medianDep = percentileCont(0.5, depVals)
	result.medianDepDelayed = percentileCont(0.5, depDelayedVals)
	result.causeCarrier = mean(causeCarrier, result.delayed)
	result.causeWeather = mean(causeWeather, result.delayed)
	result.causeNAS = mean(causeNAS, result.delayed)
	result.causeSecurity = mean(causeSecurity, result.delayed)
	result.causeLate = mean(causeLate, result.delayed)

	return result
}

func aggregateCarrierSeparate(flights []carrierFlight, state string) carrierPassResult {
	matched := make([]carrierFlight, 0)

	for _, flight := range flights {
		if carrierStateMatch(flight, state) {
			matched = append(matched, flight)
		}
	}

	var result carrierPassResult

	result.routes = map[string]routeCounts{}
	result.airports = map[string]outcomeCounts{}

	var arrSum, arrDelayedSum, depSum, depDelayedSum float64

	var arrN, arrDelayedN, depN, depDelayedN int

	var causeCarrier, causeWeather, causeNAS, causeSecurity, causeLate float64

	for _, flight := range matched {
		isCancelled, isDiverted, isDelayed := classifyCarrierFlight(flight)
		addOutcome(&result.outcomeCounts, isCancelled, isDiverted, isDelayed)

		if !isCancelled && !isDiverted && flight.arr != nil {
			arrSum += *flight.arr
			arrN++
		}

		if isDelayed && flight.arr != nil {
			arrDelayedSum += *flight.arr
			arrDelayedN++
		}

		if !isCancelled && !isDiverted && flight.dep != nil {
			depSum += *flight.dep
			depN++
		}

		if isDelayed && flight.dep != nil {
			depDelayedSum += *flight.dep
			depDelayedN++
		}

		if !isDelayed {
			continue
		}

		causeCarrier += coalesceDelay(flight.carrier)
		causeWeather += coalesceDelay(flight.weather)
		causeNAS += coalesceDelay(flight.nas)
		causeSecurity += coalesceDelay(flight.security)
		causeLate += coalesceDelay(flight.late)
		addCauseShare(&result, primaryCause(flight))
	}

	result.avgArr = mean(arrSum, arrN)
	result.avgArrDelayed = mean(arrDelayedSum, arrDelayedN)
	result.avgDep = mean(depSum, depN)
	result.avgDepDelayed = mean(depDelayedSum, depDelayedN)
	result.causeCarrier = mean(causeCarrier, result.delayed)
	result.causeWeather = mean(causeWeather, result.delayed)
	result.causeNAS = mean(causeNAS, result.delayed)
	result.causeSecurity = mean(causeSecurity, result.delayed)
	result.causeLate = mean(causeLate, result.delayed)
	result.medianArr = percentileCont(0.5, subsetDelays(matched, false, true))
	result.medianArrDelayed = percentileCont(0.5, subsetDelays(matched, true, true))
	result.medianDep = percentileCont(0.5, subsetDelays(matched, false, false))
	result.medianDepDelayed = percentileCont(0.5, subsetDelays(matched, true, false))

	for _, flight := range matched {
		isCancelled, isDiverted, isDelayed := classifyCarrierFlight(flight)
		addRouteCount(result.routes, flight, isCancelled, isDiverted, isDelayed)
	}

	for _, flight := range matched {
		if state != "" && flight.originState != state {
			continue
		}

		isCancelled, isDiverted, isDelayed := classifyCarrierFlight(flight)
		addAirportCount(result.airports, flight.origin, isCancelled, isDiverted, isDelayed)
	}

	for _, flight := range matched {
		if state != "" && flight.destState != state {
			continue
		}

		isCancelled, isDiverted, isDelayed := classifyCarrierFlight(flight)
		addAirportCount(result.airports, flight.dest, isCancelled, isDiverted, isDelayed)
	}

	return result
}

func subsetDelays(flights []carrierFlight, delayedOnly, arrival bool) []float64 {
	out := make([]float64, 0)

	for _, flight := range flights {
		isCancelled, isDiverted, isDelayed := classifyCarrierFlight(flight)
		if delayedOnly {
			if !isDelayed {
				continue
			}
		} else if isCancelled || isDiverted {
			continue
		}

		var value *float64
		if arrival {
			value = flight.arr
		} else {
			value = flight.dep
		}

		if value == nil {
			continue
		}

		out = append(out, *value)
	}

	return out
}

func addDelay(sum *float64, n *int, values *[]float64, include bool, value *float64) {
	if !include || value == nil {
		return
	}

	*sum += *value
	*n++

	*values = append(*values, *value)
}

func addOutcome(counts *outcomeCounts, isCancelled, isDiverted, isDelayed bool) {
	counts.flights++

	if !isCancelled && !isDiverted && !isDelayed {
		counts.onTime++
	}

	if isDelayed {
		counts.delayed++
	}

	if isCancelled {
		counts.cancelled++
	}

	if isDiverted {
		counts.diverted++
	}
}

func addCauseShare(result *carrierPassResult, cause string) {
	switch cause {
	case "carrier":
		result.shareCarrier++
	case "weather":
		result.shareWeather++
	case "nas":
		result.shareNAS++
	case "security":
		result.shareSecurity++
	case "late_aircraft":
		result.shareLate++
	case "unattributed":
		result.shareUnattributed++
	}
}

func addRouteCount(routes map[string]routeCounts, flight carrierFlight, isCancelled, isDiverted, isDelayed bool) {
	key := routeKey(flight.origin, flight.dest)
	row := routes[key]
	row.origin = flight.origin
	row.dest = flight.dest
	addOutcome(&row.outcomeCounts, isCancelled, isDiverted, isDelayed)
	routes[key] = row
}

func addAirportCount(airports map[string]outcomeCounts, code string, isCancelled, isDiverted, isDelayed bool) {
	row := airports[code]
	addOutcome(&row, isCancelled, isDiverted, isDelayed)
	airports[code] = row
}

func classifyCarrierFlight(flight carrierFlight) (isCancelled, isDiverted, isDelayed bool) {
	isCancelled = flight.cancelled >= 1
	isDiverted = flight.cancelled < 1 && flight.diverted >= 1
	isDelayed = flight.cancelled < 1 && flight.diverted < 1 && flight.arrDel15 >= 1

	return isCancelled, isDiverted, isDelayed
}

func primaryCause(flight carrierFlight) string {
	_, _, delayed := classifyCarrierFlight(flight)
	if !delayed {
		return ""
	}

	carrier := coalesceDelay(flight.carrier)
	weather := coalesceDelay(flight.weather)
	nas := coalesceDelay(flight.nas)
	security := coalesceDelay(flight.security)
	late := coalesceDelay(flight.late)
	maxCause := carrier

	if weather > maxCause {
		maxCause = weather
	}

	if nas > maxCause {
		maxCause = nas
	}

	if security > maxCause {
		maxCause = security
	}

	if late > maxCause {
		maxCause = late
	}

	switch {
	case maxCause == 0:
		return "unattributed"
	case late == maxCause:
		return "late_aircraft"
	case carrier == maxCause:
		return "carrier"
	case nas == maxCause:
		return "nas"
	case weather == maxCause:
		return "weather"
	default:
		return "security"
	}
}

func carrierStateMatch(flight carrierFlight, state string) bool {
	if state == "" {
		return true
	}

	return flight.originState == state || flight.destState == state
}

func routeKey(origin, dest string) string {
	return origin + "\x00" + dest
}

func coalesceDelay(value *float64) float64 {
	if value == nil {
		return 0
	}

	return *value
}

func mean(sum float64, n int) float64 {
	if n == 0 {
		return 0
	}

	return sum / float64(n)
}

func tinyCarrierFlights() []carrierFlight {
	return []carrierFlight{
		{origin: testTravelOrigin, dest: testAirportLAX, originState: "IL", destState: "CA", arr: floatPtr(-5), dep: floatPtr(-3)},
		{origin: testTravelOrigin, dest: testAirportLAX, originState: "IL", destState: "CA", arrDel15: 1, arr: floatPtr(20), dep: floatPtr(15), carrier: floatPtr(20)},
		{origin: testTravelOrigin, dest: testAirportLAX, originState: "IL", destState: "CA", arrDel15: 1, arr: floatPtr(40), carrier: floatPtr(10), late: floatPtr(10)},
		{origin: testTravelOrigin, dest: testAirportLAX, originState: "IL", destState: "CA", cancelled: 1, diverted: 1, arr: floatPtr(999), dep: floatPtr(999)},
		{origin: testAirportSFO, dest: testTravelOrigin, originState: "CA", destState: "IL", diverted: 1, arr: floatPtr(50), dep: floatPtr(50)},
		{origin: testAirportMDW, dest: testTravelOrigin, originState: "IL", destState: "IL", arrDel15: 1, dep: floatPtr(8)},
		{origin: testAirportLAX, dest: testAirportSFO, originState: "CA", destState: "CA", arr: floatPtr(1), dep: floatPtr(1)},
	}
}

func causeBucketFlights() []carrierFlight {
	delayed := func(carrier, weather, nas, security, late *float64) carrierFlight {
		return carrierFlight{
			origin: testTravelOrigin, dest: testAirportLAX, originState: "IL", destState: "CA",
			arrDel15: 1, arr: floatPtr(10), dep: floatPtr(10),
			carrier: carrier, weather: weather, nas: nas, security: security, late: late,
		}
	}

	return []carrierFlight{
		delayed(floatPtr(5), nil, nil, nil, nil),
		delayed(nil, floatPtr(4), nil, nil, nil),
		delayed(nil, nil, floatPtr(3), nil, nil),
		delayed(nil, nil, nil, floatPtr(2), nil),
		delayed(nil, nil, nil, nil, floatPtr(9)),
		delayed(nil, nil, nil, nil, nil),
		delayed(floatPtr(7), nil, nil, nil, floatPtr(7)),
	}
}

func generatedCarrierFlights() []carrierFlight {
	var flights []carrierFlight

	addGeneratedRoute(&flights, testAirportAAA, testAirportBBB, "IL", "CA", 40, 32, 4, 2)
	addGeneratedRoute(&flights, "CCC", "DDD", "CO", "AZ", 38, 20, 10, 4)
	addGeneratedRoute(&flights, "EEE", "FFF", "WA", "OR", 36, 18, 10, 4)
	addGeneratedRoute(&flights, "GGG", "HHH", "TX", "TX", 34, 10, 14, 5)
	addGeneratedRoute(&flights, "III", "JJJ", "FL", "GA", 33, 8, 15, 5)
	addGeneratedRoute(&flights, "KKK", "LLL", "NV", "UT", 32, 6, 16, 5)
	addGeneratedRoute(&flights, "MMM", "NNN", "MA", "NY", 31, 4, 17, 5)
	addGeneratedRoute(&flights, "OOO", "PPP", "MI", "MI", 30, 3, 17, 5)
	addGeneratedRoute(&flights, testAirportQQQ, testAirportRRR, "NC", "SC", 29, 29, 0, 0)
	addGeneratedRoute(&flights, "SSS", "TTT", "CA", "WA", 40, 10, 20, 5)
	addGeneratedRoute(&flights, "UUU", testAirportAAA, "WA", "IL", 30, 15, 10, 3)

	return flights
}

func addGeneratedRoute(flights *[]carrierFlight, origin, dest, originState, destState string, n, onTime, delayed, cancelled int) {
	for i := range n {
		flight := carrierFlight{origin: origin, dest: dest, originState: originState, destState: destState}

		switch {
		case i < onTime:
			flight.arr = floatPtr(float64(i - onTime/2))
			flight.dep = floatPtr(float64(i - 3))

			if i == 0 && origin == testAirportAAA {
				flight.dep = nil
			}
		case i < onTime+delayed:
			flight.arrDel15 = 1
			delay := float64(15 + i)
			flight.arr = floatPtr(delay)
			flight.dep = floatPtr(float64(5 + i))

			if i == onTime && origin == testAirportAAA {
				flight.arr = nil
			}

			switch (i - onTime) % 5 {
			case 0:
				flight.carrier = floatPtr(delay)
			case 1:
				flight.weather = floatPtr(delay)
			case 2:
				flight.nas = floatPtr(delay)
			case 3:
				flight.security = floatPtr(delay)
			default:
				flight.late = floatPtr(delay)
				flight.carrier = floatPtr(delay)
			}
		case i < onTime+delayed+cancelled:
			flight.cancelled = 1
			flight.diverted = 1
			flight.arr = floatPtr(999)
			flight.dep = floatPtr(999)
		default:
			flight.diverted = 1
			flight.arr = floatPtr(40)
			flight.dep = floatPtr(40)
		}

		*flights = append(*flights, flight)
	}
}

func cancelledOnlyFlight() carrierFlight {
	return carrierFlight{
		origin: testTravelOrigin, dest: testAirportLAX, originState: "IL", destState: "CA",
		cancelled: 1, arr: floatPtr(100), dep: floatPtr(100),
	}
}

func singleArrivalFlight() carrierFlight {
	return carrierFlight{
		origin: testTravelOrigin, dest: testAirportLAX, originState: "IL", destState: "CA",
		arr: floatPtr(4), dep: floatPtr(7),
	}
}
