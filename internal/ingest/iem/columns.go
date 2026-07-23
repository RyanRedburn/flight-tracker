package iem

import "strings"

const (
	colYear      = "year"
	colMonth     = "month"
	colStation   = "station"
	colValid     = "valid"
	jsonKeyMonth = "month"
	jsonKeyRows  = "rows_imported"
)

// ObservationColumns is the IEM CSV column order (excluding year/month partition keys).
var ObservationColumns = []string{
	colStation,
	colValid,
	"tmpf",
	"dwpf",
	"relh",
	"drct",
	"sknt",
	"gust",
	"vsby",
	"skyc1",
	"skyc2",
	"skyc3",
	"skyl1",
	"skyl2",
	"skyl3",
	"wxcodes",
	"p01i",
	"alti",
	"mslp",
	"metar",
}

// DBColumns is the weather_observations insert column order.
var DBColumns = append([]string{colYear, colMonth}, ObservationColumns...)

func csvHeaderToColumn(header string) string {
	return strings.TrimSpace(header)
}
