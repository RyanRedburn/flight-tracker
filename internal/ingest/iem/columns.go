package iem

import "strings"

const (
	colYear      = "year"
	colMonth     = "month"
	colStation   = "station"
	colValid     = "valid"
	colTmpf      = "tmpf"
	colDwpf      = "dwpf"
	colRelh      = "relh"
	colDrct      = "drct"
	colSknt      = "sknt"
	colGust      = "gust"
	colVsby      = "vsby"
	colSkyc1     = "skyc1"
	colSkyc2     = "skyc2"
	colSkyc3     = "skyc3"
	colSkyl1     = "skyl1"
	colSkyl2     = "skyl2"
	colSkyl3     = "skyl3"
	colWxcodes   = "wxcodes"
	colP01i      = "p01i"
	colAlti      = "alti"
	colMslp      = "mslp"
	colMetar     = "metar"
	jsonKeyMonth = "month"
	jsonKeyRows  = "rows_imported"
)

// ObservationColumns is the IEM CSV column order (excluding year/month partition keys).
var ObservationColumns = []string{
	colStation,
	colValid,
	colTmpf,
	colDwpf,
	colRelh,
	colDrct,
	colSknt,
	colGust,
	colVsby,
	colSkyc1,
	colSkyc2,
	colSkyc3,
	colSkyl1,
	colSkyl2,
	colSkyl3,
	colWxcodes,
	colP01i,
	colAlti,
	colMslp,
	colMetar,
}

// dataVars are IEM `data=` columns (station/valid are always returned).
var dataVars = ObservationColumns[2:]

// DBColumns is the weather_observations insert column order.
var DBColumns = append([]string{colYear, colMonth}, ObservationColumns...)

func csvHeaderToColumn(header string) string {
	return strings.TrimSpace(header)
}
