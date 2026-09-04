package iem

import (
	"sort"
	"strconv"
	"time"

	"github.com/RyanRedburn/flight-tracker/internal/store"
)

func encodeWeatherStations(catalog map[string]Station) [][]string {
	sids := make([]string, 0, len(catalog))
	for sid := range catalog {
		sids = append(sids, sid)
	}

	sort.Strings(sids)

	rows := make([][]string, 0, len(sids))
	for _, sid := range sids {
		station := catalog[sid]
		rows = append(rows, []string{
			station.SID,
			station.Network,
			station.Name,
			station.TzName,
			formatFloatPtr(station.Latitude),
			formatFloatPtr(station.Longitude),
			station.ArchiveBegin,
		})
	}

	return rows
}

func encodeAirportWeatherStations(rows []store.AirportWeatherStation, updatedAt time.Time) [][]string {
	out := make([][]string, 0, len(rows))
	ts := updatedAt.UTC().Format(time.RFC3339)

	for _, row := range rows {
		out = append(out, []string{
			row.AirportCode,
			row.IEMSID,
			row.TzName,
			strconv.FormatBool(row.Matched),
			ts,
		})
	}

	return out
}

func formatFloatPtr(v *float64) string {
	if v == nil {
		return ""
	}

	return strconv.FormatFloat(*v, 'f', -1, 64)
}
