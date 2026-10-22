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
	phraseLowConfidence     = "low confidence"
	phraseUnknownConfidence = "unknown confidence"

	nounMonth      = "month"
	nounCarrier    = "carrier"
	nounListedTime = "listed time"
)

type scoredItem struct {
	label      string
	percent    int
	sampleSize int
	confidence string
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

	if confidence == model.ConfidenceHigh {
		return model.SampleReasonSufficient
	}

	return model.SampleReasonInsufficientSample
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
	ranked, thin, none := splitByConfidence(items)
	summary := noneEnough(noun, route)

	switch len(ranked) {
	case 0:
	case 1:
		summary = onlyEnough(noun, route, ranked[0], detail)
	default:
		summary = rankedComparison(route, ranked, detail)
	}

	if detail {
		return appendNote(summary, thinSentence(thin), noneSentence(none)), comparisonConfidence(ranked, thin, none)
	}

	return appendNote(summary, gapSentence(thin, none)), comparisonConfidence(ranked, thin, none)
}

func comparisonConfidence(ranked, thin, none []scoredItem) string {
	if len(ranked) > 0 {
		return model.ConfidenceHigh
	}

	if len(none) > 0 {
		return model.ConfidenceUnknown
	}

	if len(thin) > 0 {
		return model.ConfidenceLow
	}

	return model.ConfidenceUnknown
}

func splitByConfidence(items []scoredItem) (ranked, thin, none []scoredItem) {
	for _, item := range items {
		switch item.confidence {
		case model.ConfidenceHigh:
			ranked = append(ranked, item)
		case model.ConfidenceLow:
			thin = append(thin, item)
		default:
			none = append(none, item)
		}
	}

	return ranked, thin, none
}

func noneEnough(noun, route string) string {
	return "No " + noun + " on " + route + " has enough data to compare."
}

func onlyEnough(noun, route string, item scoredItem, detail bool) string {
	if !detail {
		return fmt.Sprintf("%s is the only %s with enough data on %s (%d%% on time).",
			item.label, noun, route, item.percent)
	}

	return fmt.Sprintf("%s is the only %s with enough data on %s (%d%% on time, %s, %s).",
		item.label, noun, route, item.percent, phraseHighConfidence, flightsPhrase([]int{item.sampleSize}))
}

func rankedComparison(route string, ranked []scoredItem, detail bool) string {
	sorted := append([]scoredItem(nil), ranked...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].percent > sorted[j].percent })

	split := 1
	for split < len(sorted) && sorted[0].percent-sorted[split].percent < routeReliabilitySamePointGap {
		split++
	}

	leaders, trailers := sorted[:split], sorted[split:]
	if len(trailers) == 0 {
		return aboutTheSameItems(route, leaders, detail)
	}

	if leaders[len(leaders)-1].percent-trailers[0].percent < routeReliabilitySamePointGap {
		return orderedItems(route, sorted, detail)
	}

	if len(leaders) == 1 {
		return fmt.Sprintf("%s is more reliable than %s on %s (%s).",
			leaders[0].label, joinLabels(trailers), route, onTimeDetail(fmt.Sprintf("%d%% vs %s", leaders[0].percent, joinItemPercents(trailers)), flightsPhrase(sampleSizes(sorted)), detail))
	}

	leaderRatesText := joinItemPercents(leaders)
	if detail {
		leaderRatesText = leaderRates(leaders)
	}

	return fmt.Sprintf("%s are about the same and more reliable than %s on %s (%s).",
		joinLabels(leaders), joinLabels(trailers), route, onTimeDetail(leaderRatesText+" vs "+joinItemPercents(trailers), flightsPhrase(sampleSizes(sorted)), detail))
}

func aboutTheSameItems(route string, leaders []scoredItem, detail bool) string {
	rates := joinItemPercents(leaders)
	if sameItemPercent(leaders) {
		rates = strconv.Itoa(leaders[0].percent) + "%"
	}

	return fmt.Sprintf("%s are about the same on %s (%s).",
		joinLabels(leaders), route, onTimeDetail(rates, flightsPhrase(sampleSizes(leaders)), detail))
}

func orderedItems(route string, sorted []scoredItem, detail bool) string {
	tied := 1
	for tied < len(sorted) && sorted[tied].percent == sorted[0].percent {
		tied++
	}

	rest := make([]string, 0, len(sorted)-tied)
	for _, item := range sorted[tied:] {
		if detail {
			rest = append(rest, fmt.Sprintf("%s (%d%%, %s, %s)", item.label, item.percent, phraseHighConfidence, flightsPhrase([]int{item.sampleSize})))
			continue
		}

		rest = append(rest, fmt.Sprintf("%s (%d%%)", item.label, item.percent))
	}

	leaders := sorted[:tied]

	verb := pluralVerb(tied)

	lead := strconv.Itoa(sorted[0].percent) + "%"
	if detail {
		lead = onTimeDetail(lead, flightsPhrase(sampleSizes(leaders)), true)
	} else {
		lead += " on time"
	}

	return fmt.Sprintf("%s %s most reliable on %s (%s), then %s.",
		joinLabels(leaders), verb, route, lead, joinAnd(rest))
}

func onTimeDetail(rates, flights string, detail bool) string {
	if !detail {
		return rates + " on time"
	}

	return rates + " on time, " + phraseHighConfidence + ", " + flights
}

func thinSentence(items []scoredItem) string {
	return confidenceSentence(items, phraseLowConfidence, "thin data")
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

func gapSentence(thin, none []scoredItem) string {
	parts := make([]string, 0, 2)
	if len(thin) > 0 {
		parts = append(parts, joinLabels(thin)+" "+haveVerb(len(thin))+" thin data")
	}

	if len(none) > 0 {
		parts = append(parts, joinLabels(none)+" "+haveVerb(len(none))+" no data")
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, " and ") + "."
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
