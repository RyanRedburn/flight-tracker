package mcp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	phraseHighConfidence    = "high confidence"
	phraseMediumConfidence  = "medium confidence"
	phraseLowConfidence     = "low confidence"
	phraseUnknownConfidence = "unknown confidence"
	phraseTooClose          = "too close to call"
	summaryNameCap          = 3

	nounMonth      = "month"
	nounCarrier    = "carrier"
	nounListedTime = "listed time"
)

type scoredItem struct {
	label      string
	percent    int
	rate       float64
	sampleSize int
	confidence string
	key        int
	hasRate    bool
}

func routeLabel(carrier, origin, dest string) string {
	if carrier == "" {
		return origin + "–" + dest
	}

	return carrier + " " + origin + "–" + dest
}

func carrierLabel(code string) string {
	if code == "" {
		return "an unknown carrier"
	}

	return code
}

func sampleReason(flights int, confidence string) string {
	if flights <= 0 {
		return model.SampleReasonEmptySample
	}

	if comparableConfidence(confidence) {
		return model.SampleReasonSufficient
	}

	return model.SampleReasonInsufficientSample
}

func comparableConfidence(confidence string) bool {
	switch confidence {
	case model.ConfidenceHigh, model.ConfidenceMedium:
		return true
	default:
		return false
	}
}

func confidencePhrase(confidence string) string {
	switch confidence {
	case model.ConfidenceHigh:
		return phraseHighConfidence
	case model.ConfidenceMedium:
		return phraseMediumConfidence
	case model.ConfidenceLow:
		return phraseLowConfidence
	default:
		return phraseUnknownConfidence
	}
}

func itemConfidence(items []scoredItem) string {
	values := make([]string, len(items))
	for i, item := range items {
		values[i] = item.confidence
	}

	return model.MinConfidence(values...)
}

// sentenceCase capitalizes a sentence that opens with a lowercase label such as "an unknown carrier".
func sentenceCase(sentence string) string {
	r, size := utf8.DecodeRuneInString(sentence)
	if r == utf8.RuneError || !unicode.IsLower(r) {
		return sentence
	}

	return string(unicode.ToUpper(r)) + sentence[size:]
}

func comparisonSummary(noun, route string, items []scoredItem, detail bool) (string, string) {
	ranked, none := splitByData(items)
	summary := noneEnough(noun, route)

	switch len(ranked) {
	case 0:
	case 1:
		summary = onlyEnough(noun, route, ranked[0], detail)
	default:
		summary = rankedComparison(noun, route, ranked, detail)
	}

	if detail {
		return appendNote(summary, noneSentence(none)), comparisonConfidence(ranked)
	}

	return appendNote(summary, gapSentence(none)), comparisonConfidence(ranked)
}

func comparisonConfidence(ranked []scoredItem) string {
	if len(ranked) > 0 {
		return itemConfidence(ranked)
	}

	return model.ConfidenceUnknown
}

func splitByData(items []scoredItem) (ranked, none []scoredItem) {
	for _, item := range items {
		if item.sampleSize > 0 && item.hasRate {
			ranked = append(ranked, item)

			continue
		}

		none = append(none, item)
	}

	return ranked, none
}

func noneEnough(noun, route string) string {
	return "No " + noun + " on " + route + " has data to compare."
}

func onlyEnough(noun, route string, item scoredItem, detail bool) string {
	if !detail {
		return fmt.Sprintf("%s is the only %s on %s (%d%% on time).",
			item.label, noun, route, item.percent)
	}

	return fmt.Sprintf("%s is the only %s on %s (%d%% on time, %s, %s).",
		item.label, noun, route, item.percent, confidencePhrase(item.confidence), flightsPhrase([]int{item.sampleSize}))
}

func rankedComparison(noun, route string, ranked []scoredItem, detail bool) string {
	sorted := sortByWilsonLower(ranked)
	if !anyItemDiffers(sorted) {
		return tooCloseItems(noun, route, sorted, detail)
	}

	if len(sorted) > 2 && allPairsDiffer(sorted) {
		return orderedItems(route, sorted, detail)
	}

	leaders, trailers := splitLeaders(sorted)
	if len(trailers) > 0 && leadersSeparated(leaders, trailers) {
		return leaderSentence(route, leaders, trailers, detail)
	}

	return mixedComparison(noun, route, sorted, detail)
}

func tooCloseItems(noun, route string, items []scoredItem, detail bool) string {
	sentence := fmt.Sprintf("%s are %s on %s (%s on time)",
		countLabel(len(items), noun), phraseTooClose, route, percentSpan(items))
	if !detail {
		return sentence + "."
	}

	return sentence + ", " + confidencePhrase(itemConfidence(items)) + ", " + flightSpan(items) + "."
}

func countLabel(n int, noun string) string {
	plural := pluralNoun(noun)
	if n == 2 {
		return "Both " + plural
	}

	return fmt.Sprintf("All %d %s", n, plural)
}

func pluralNoun(noun string) string {
	switch noun {
	case nounMonth:
		return "months"
	case nounCarrier:
		return "carriers"
	case nounListedTime:
		return "times"
	case "day":
		return "days"
	case "hour":
		return "hours"
	default:
		return noun + "s"
	}
}

func leaderSentence(route string, leaders, trailers []scoredItem, detail bool) string {
	compared := append(append([]scoredItem{}, leaders...), trailers...)
	confidence := confidencePhrase(itemConfidence(compared))
	namedLeaders, otherLeaders := capNamed(leaders)
	namedLeaders = sortByRateDesc(namedLeaders)
	namedTrailers, otherTrailers := tailAndOthers(trailers)
	named := append(append([]scoredItem{}, namedLeaders...), namedTrailers...)
	flights := flightsPhrase(sampleSizes(named))

	others := append(append([]scoredItem{}, otherLeaders...), otherTrailers...)

	if len(namedLeaders) == 1 {
		sentence := fmt.Sprintf("%s is more reliable than %s on %s (%s).",
			namedLeaders[0].label, joinLabels(namedTrailers), route, onTimeDetail(fmt.Sprintf("%d%% vs %s", namedLeaders[0].percent, joinItemPercents(namedTrailers)), flights, confidence, detail))

		return appendOthers(sentence, others)
	}

	leaderRatesText := joinItemPercents(namedLeaders)
	if detail {
		leaderRatesText = leaderRates(namedLeaders)
	}

	sentence := fmt.Sprintf("%s are %s and more reliable than %s on %s (%s).",
		joinLabels(namedLeaders), phraseTooClose, joinLabels(namedTrailers), route, onTimeDetail(leaderRatesText+" vs "+joinItemPercents(namedTrailers), flights, confidence, detail))

	return appendOthers(sentence, others)
}

func mixedComparison(noun, route string, sorted []scoredItem, detail bool) string {
	best := sorted[0]
	beaten := make([]scoredItem, 0, len(sorted))
	tied := make([]scoredItem, 0, len(sorted))

	for _, item := range sorted[1:] {
		if itemDiffers(best, item) {
			beaten = append(beaten, item)

			continue
		}

		tied = append(tied, item)
	}

	if len(beaten) == 0 {
		return tooCloseItems(noun, route, sorted, detail)
	}

	confidence := confidencePhrase(itemConfidence(sorted))
	namedBeaten, otherBeaten := tailAndOthers(beaten)
	compared := append([]scoredItem{best}, namedBeaten...)
	lead := fmt.Sprintf("%s is more reliable than %s on %s (%s).",
		best.label, joinLabels(namedBeaten), route, onTimeDetail(fmt.Sprintf("%d%% vs %s", best.percent, joinItemPercents(namedBeaten)), flightsPhrase(sampleSizes(compared)), confidence, detail))
	lead = appendOthers(lead, otherBeaten)

	if len(tied) == 0 {
		return lead
	}

	return lead + " " + tiedAgainst(tied, best.label)
}

func tiedAgainst(tied []scoredItem, against string) string {
	named, others := capNamed(tied)
	named = sortByRateDesc(named)
	labels := make([]string, len(named))

	for i, item := range named {
		labels[i] = fmt.Sprintf("%s (%d%%)", item.label, item.percent)
	}

	sentence := sentenceCase(fmt.Sprintf("%s %s %s against %s.", joinAnd(labels), pluralVerb(len(named)), phraseTooClose, against))

	return appendOthers(sentence, others)
}

func orderedItems(route string, sorted []scoredItem, detail bool) string {
	tied := 1
	for tied < len(sorted) && !itemDiffers(sorted[0], sorted[tied]) {
		tied++
	}

	namedCount := min(tied, summaryNameCap)
	leaders := sortByRateDesc(sorted[:namedCount])
	worst, others := tailAndOthers(sorted[tied:])

	if tied > summaryNameCap {
		others = append(append([]scoredItem{}, sorted[namedCount:tied]...), others...)
	}

	rest := make([]string, 0, len(worst))

	for _, item := range worst {
		if detail {
			rest = append(rest, fmt.Sprintf("%s (%d%%, %s, %s)", item.label, item.percent, confidencePhrase(item.confidence), flightsPhrase([]int{item.sampleSize})))

			continue
		}

		rest = append(rest, fmt.Sprintf("%s (%d%%)", item.label, item.percent))
	}

	verb := pluralVerb(len(leaders))
	lead := strconv.Itoa(leaders[0].percent) + "%"

	if detail {
		lead = onTimeDetail(lead, flightsPhrase(sampleSizes(leaders)), confidencePhrase(itemConfidence(sorted)), true)
	} else {
		lead += " on time"
	}

	sentence := fmt.Sprintf("%s %s most reliable on %s (%s), then %s.",
		joinLabels(leaders), verb, route, lead, joinAnd(rest))

	return appendOthers(sentence, others)
}

func capNamed(items []scoredItem) (named, others []scoredItem) {
	if len(items) <= summaryNameCap {
		return items, nil
	}

	return items[:summaryNameCap], items[summaryNameCap:]
}

// tailAndOthers keeps the lowest Wilson upper bounds, then lists that slice by rate.
func tailAndOthers(items []scoredItem) (tail, others []scoredItem) {
	ordered := sortByWilsonUpper(items)
	tail, others = capNamed(ordered)

	return sortByRateDesc(tail), others
}

func appendOthers(sentence string, others []scoredItem) string {
	if len(others) == 0 {
		return sentence
	}

	return sentence + " " + othersPhrase(others) + "."
}

func othersPhrase(items []scoredItem) string {
	noun := "others"
	if len(items) == 1 {
		noun = "other"
	}

	return fmt.Sprintf("%d %s, %s", len(items), noun, percentSpan(items))
}

func percentSpan(items []scoredItem) string {
	lo, hi := items[0].percent, items[0].percent

	for _, item := range items[1:] {
		lo = min(lo, item.percent)
		hi = max(hi, item.percent)
	}

	if lo == hi {
		return strconv.Itoa(lo) + "%"
	}

	return strconv.Itoa(lo) + "–" + strconv.Itoa(hi) + "%"
}

func flightSpan(items []scoredItem) string {
	lo, hi := items[0].sampleSize, items[0].sampleSize

	for _, item := range items[1:] {
		lo = min(lo, item.sampleSize)
		hi = max(hi, item.sampleSize)
	}

	if lo == hi {
		word := "flights"
		if lo == 1 {
			word = "flight"
		}

		return strconv.Itoa(lo) + " " + word + " each"
	}

	return strconv.Itoa(lo) + "–" + strconv.Itoa(hi) + " flights each"
}

func onTimeDetail(rates, flights, confidence string, detail bool) string {
	if !detail {
		return rates + " on time"
	}

	return rates + " on time, " + confidence + ", " + flights
}

func noneSentence(items []scoredItem) string {
	return confidenceSentence(items, phraseUnknownConfidence, "no reliable on-time rate")
}

func confidenceSentence(items []scoredItem, confidence, tail string) string {
	if len(items) == 0 {
		return ""
	}

	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = fmt.Sprintf("%s (%s, %s)", item.label, confidence, flightsPhrase([]int{item.sampleSize}))
	}

	return joinAnd(parts) + " " + haveVerb(len(items)) + " " + tail + "."
}

func gapSentence(none []scoredItem) string {
	if len(none) == 0 {
		return ""
	}

	return joinLabels(none) + " " + haveVerb(len(none)) + " no data."
}

func appendNote(base string, notes ...string) string {
	parts := make([]string, 0, 1+len(notes))
	if base != "" {
		parts = append(parts, sentenceCase(base))
	}

	for _, note := range notes {
		if note == "" {
			continue
		}

		parts = append(parts, sentenceCase(note))
	}

	return strings.Join(parts, " ")
}

func haveVerb(n int) string {
	if n == 1 {
		return "has"
	}

	return "have"
}

func pluralVerb(n int) string {
	if n == 1 {
		return "is"
	}

	return "are"
}

func leaderRates(leaders []scoredItem) string {
	if sameItemPercent(leaders) {
		return strconv.Itoa(leaders[0].percent) + "%"
	}

	return joinItemPercents(leaders)
}

func sortByWilsonLower(items []scoredItem) []scoredItem {
	return sortByWilson(items, false)
}

func sortByWilsonUpper(items []scoredItem) []scoredItem {
	return sortByWilson(items, true)
}

func sortByWilson(items []scoredItem, upper bool) []scoredItem {
	sorted := append([]scoredItem(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		leftLower, leftUpper := model.WilsonBounds(sorted[i].rate, nil, sorted[i].sampleSize)
		rightLower, rightUpper := model.WilsonBounds(sorted[j].rate, nil, sorted[j].sampleSize)
		left, right := leftLower, rightLower

		if upper {
			left, right = leftUpper, rightUpper
		}

		if left != right {
			if upper {
				return left < right
			}

			return left > right
		}

		if sorted[i].sampleSize != sorted[j].sampleSize {
			return sorted[i].sampleSize > sorted[j].sampleSize
		}

		if sorted[i].key != sorted[j].key {
			return sorted[i].key < sorted[j].key
		}

		return sorted[i].label < sorted[j].label
	})

	return sorted
}

func sortByRateDesc(items []scoredItem) []scoredItem {
	sorted := append([]scoredItem(nil), items...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].rate != sorted[j].rate {
			return sorted[i].rate > sorted[j].rate
		}

		if sorted[i].sampleSize != sorted[j].sampleSize {
			return sorted[i].sampleSize > sorted[j].sampleSize
		}

		if sorted[i].key != sorted[j].key {
			return sorted[i].key < sorted[j].key
		}

		return sorted[i].label < sorted[j].label
	})

	return sorted
}

func wilsonLeader(items []scoredItem) scoredItem {
	ordered := sortByWilsonLower(items)

	return ordered[0]
}

func wilsonAnchor(items []scoredItem) scoredItem {
	ordered := sortByWilsonUpper(items)

	return ordered[0]
}

func exceptKey(items []scoredItem, key int) []scoredItem {
	rest := make([]scoredItem, 0, len(items))

	for _, item := range items {
		if item.key == key {
			continue
		}

		rest = append(rest, item)
	}

	return rest
}

func itemDiffers(a, b scoredItem) bool {
	return model.ProportionsDiffer(a.rate, a.sampleSize, b.rate, b.sampleSize)
}

func anyItemDiffers(items []scoredItem) bool {
	for i := range items {
		for j := i + 1; j < len(items); j++ {
			if itemDiffers(items[i], items[j]) {
				return true
			}
		}
	}

	return false
}

func allPairsDiffer(items []scoredItem) bool {
	if len(items) < 2 {
		return false
	}

	for i := range items {
		for j := i + 1; j < len(items); j++ {
			if !itemDiffers(items[i], items[j]) {
				return false
			}
		}
	}

	return true
}

func splitLeaders(sorted []scoredItem) (leaders, trailers []scoredItem) {
	for _, item := range sorted {
		if !itemDiffers(sorted[0], item) {
			leaders = append(leaders, item)
			continue
		}

		trailers = append(trailers, item)
	}

	return leaders, trailers
}

func leadersSeparated(leaders, trailers []scoredItem) bool {
	for i := range leaders {
		for j := i + 1; j < len(leaders); j++ {
			if itemDiffers(leaders[i], leaders[j]) {
				return false
			}
		}
	}

	for _, leader := range leaders {
		for _, trailer := range trailers {
			if !itemDiffers(leader, trailer) {
				return false
			}
		}
	}

	return true
}

func sameItemPercent(items []scoredItem) bool {
	for _, item := range items[1:] {
		if item.percent != items[0].percent {
			return false
		}
	}

	return true
}

func sampleSizes(items []scoredItem) []int {
	sizes := make([]int, len(items))
	for i, item := range items {
		sizes[i] = item.sampleSize
	}

	return sizes
}

func joinLabels(items []scoredItem) string {
	labels := make([]string, len(items))
	for i, item := range items {
		labels[i] = item.label
	}

	return joinAnd(labels)
}

func joinItemPercents(items []scoredItem) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = strconv.Itoa(item.percent) + "%"
	}

	return joinAnd(parts)
}

func flightsPhrase(sizes []int) string {
	parts := make([]string, len(sizes))
	for i, n := range sizes {
		parts[i] = strconv.Itoa(n)
	}

	word := "flights"
	if len(sizes) == 1 && sizes[0] == 1 {
		word = "flight"
	}

	return joinAnd(parts) + " " + word
}

func emptyWindowSummary(kind, route, start, end string) string {
	return fmt.Sprintf("No %s for %s between %s and %s, so the confidence is unknown.", kind, route, start, end)
}

func monthLabel(month int) string {
	if month < 1 || month > 12 {
		return "month " + strconv.Itoa(month)
	}

	return monthName(month)
}

func monthName(month int) string {
	names := [12]string{
		"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	}

	return names[month-1]
}
