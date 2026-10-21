package mcp

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	phraseHighConfidence    = "high confidence"
	phraseLowConfidence     = "low confidence"
	phraseUnknownConfidence = "unknown confidence"

	nounMonth   = "month"
	nounCarrier = "carrier"
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
		return "unknown carrier"
	}

	return code
}

func sampleReasonForConfidence(confidence string) string {
	switch confidence {
	case model.ConfidenceHigh:
		return model.SampleReasonSufficient
	case model.ConfidenceLow:
		return model.SampleReasonInsufficientSample
	default:
		return model.SampleReasonEmptySample
	}
}

func comparisonSummary(noun, route string, items []scoredItem) (string, string) {
	ranked, thin, none := splitByConfidence(items)
	summary := noneEnough(noun, route)

	switch len(ranked) {
	case 0:
	case 1:
		summary = onlyEnough(noun, route, ranked[0])
	default:
		summary = rankedComparison(route, ranked)
	}

	return appendNote(summary, thinSentence(thin), noneSentence(none)), comparisonConfidence(ranked, thin, none)
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

func onlyEnough(noun, route string, item scoredItem) string {
	return fmt.Sprintf("%s is the only %s with enough data on %s (%d%% on time, %s, %s).",
		item.label, noun, route, item.percent, phraseHighConfidence, flightsPhrase([]int{item.sampleSize}))
}

func rankedComparison(route string, ranked []scoredItem) string {
	sorted := append([]scoredItem(nil), ranked...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].percent > sorted[j].percent })

	split := 1
	for split < len(sorted) && sorted[0].percent-sorted[split].percent < routeReliabilitySamePointGap {
		split++
	}

	leaders, trailers := sorted[:split], sorted[split:]
	if len(trailers) == 0 {
		return aboutTheSameItems(route, leaders)
	}

	if leaders[len(leaders)-1].percent-trailers[0].percent < routeReliabilitySamePointGap {
		return orderedItems(route, sorted)
	}

	if len(leaders) == 1 {
		return fmt.Sprintf("%s is more reliable than %s on %s (%d%% vs %s on time, %s, %s).",
			leaders[0].label, joinLabels(trailers), route, leaders[0].percent, joinItemPercents(trailers), phraseHighConfidence, flightsPhrase(sampleSizes(sorted)))
	}

	return fmt.Sprintf("%s are about the same and more reliable than %s on %s (%s vs %s on time, %s, %s).",
		joinLabels(leaders), joinLabels(trailers), route, leaderRates(leaders), joinItemPercents(trailers), phraseHighConfidence, flightsPhrase(sampleSizes(sorted)))
}

func aboutTheSameItems(route string, leaders []scoredItem) string {
	if sameItemPercent(leaders) {
		return fmt.Sprintf("%s are about the same on %s (%d%% on time, %s, %s).",
			joinLabels(leaders), route, leaders[0].percent, phraseHighConfidence, flightsPhrase(sampleSizes(leaders)))
	}

	return fmt.Sprintf("%s are about the same on %s (%s on time, %s, %s).",
		joinLabels(leaders), route, joinItemPercents(leaders), phraseHighConfidence, flightsPhrase(sampleSizes(leaders)))
}

func orderedItems(route string, sorted []scoredItem) string {
	tied := 1
	for tied < len(sorted) && sorted[tied].percent == sorted[0].percent {
		tied++
	}

	rest := make([]string, 0, len(sorted)-tied)
	for _, item := range sorted[tied:] {
		rest = append(rest, fmt.Sprintf("%s (%d%%, %s, %s)", item.label, item.percent, phraseHighConfidence, flightsPhrase([]int{item.sampleSize})))
	}

	leaders := sorted[:tied]

	verb := "is"
	if tied > 1 {
		verb = "are"
	}

	return fmt.Sprintf("%s %s most reliable on %s (%d%% on time, %s, %s), then %s.",
		joinLabels(leaders), verb, route, sorted[0].percent, phraseHighConfidence, flightsPhrase(sampleSizes(leaders)), joinAnd(rest))
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

func appendNote(base string, notes ...string) string {
	parts := make([]string, 0, 1+len(notes))
	if base != "" {
		parts = append(parts, base)
	}

	for _, note := range notes {
		if note == "" {
			continue
		}

		parts = append(parts, note)
	}

	return strings.Join(parts, " ")
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
