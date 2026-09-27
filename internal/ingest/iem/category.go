package iem

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

const (
	// Visibility above 10 SM is the usual METAR "unlimited" report and is clamped
	// to 10 before category thresholds. Negative visibility is treated as missing.
	weatherVisibilityCapSM = 10
	// Ceiling heights outside [0, 100000] ft are ignored.
	weatherCeilingMaxFt = 100000
	// Wind at or above this speed (kt), using the greater of gust and sknt, is WINDY
	// when no higher-priority category matched. Speeds outside [0, 200] kt are ignored.
	weatherWindyMinKt = 25
	weatherWindMaxKt  = 200

	weatherLIFRCeilingFt = 500
	weatherIFRCeilingFt  = 1000
	weatherMVFRCeilingFt = 3000
	weatherLIFRVisSM     = 1
	weatherIFRVisSM      = 3
	weatherMVFRVisSM     = 5

	colCeilingFt = "ceiling_ft"
	colCategory  = "category"

	coverBKN = "BKN"
	coverOVC = "OVC"
	coverVV  = "VV"
	coverFEW = "FEW"
	coverSCT = "SCT"
)

// precipPhenomena are present-weather tokens that count as PRECIP after thunder
// and flight-category rules. BR-only mist is not in this list. FZ* is handled
// separately so freezing fog is precip even without a RA/DZ/SN token.
var precipPhenomena = []string{"RA", "DZ", "SN", "SG", "IC", "PL", "GR", "GS", "UP"}

// MetarFields is the slice of an observation used to assign one category.
type MetarFields struct {
	Vsby     *float64
	Sknt     *float64
	Gust     *float64
	SkyCover [3]string
	SkyLevel [3]*float64
	WxCodes  string
}

// ClassifyMetar returns ceiling_ft (lowest BKN/OVC/VV across the three layers,
// nil when none) and one category. First match wins:
// THUNDER, LIFR, IFR, MVFR, WINDY, PRECIP, UNKNOWN, VFR_FAIR.
// UNKNOWN is visibility, ceiling, and wind all missing, and only applies when
// thunder and precip codes did not already match. Empty wxcodes means no phenomena.
func ClassifyMetar(fields MetarFields) (ceilingFt *int, category string) {
	ceilingFt = lowestCeiling(fields.SkyCover, fields.SkyLevel)
	vis := clampVisibility(fields.Vsby)
	wind := maxWindKt(fields.Sknt, fields.Gust)
	thunder, precip := scanWxCodes(fields.WxCodes)

	return ceilingFt, classifyCategory(ceilingFt, vis, wind, thunder, precip)
}

func classifyCategory(ceiling *int, vis, wind *float64, thunder, precip bool) string {
	if thunder {
		return model.WeatherCategoryThunder
	}

	if (ceiling != nil && *ceiling < weatherLIFRCeilingFt) || (vis != nil && *vis < weatherLIFRVisSM) {
		return model.WeatherCategoryLIFR
	}

	if (ceiling != nil && *ceiling < weatherIFRCeilingFt) || (vis != nil && *vis < weatherIFRVisSM) {
		return model.WeatherCategoryIFR
	}

	if (ceiling != nil && *ceiling <= weatherMVFRCeilingFt) || (vis != nil && *vis <= weatherMVFRVisSM) {
		return model.WeatherCategoryMVFR
	}

	if wind != nil && *wind >= weatherWindyMinKt {
		return model.WeatherCategoryWindy
	}

	if precip {
		return model.WeatherCategoryPrecip
	}

	if vis == nil && ceiling == nil && wind == nil {
		return model.WeatherCategoryUnknown
	}

	return model.WeatherCategoryVFRFair
}

func lowestCeiling(covers [3]string, levels [3]*float64) *int {
	var best *int

	for i := range covers {
		ft, ok := ceilingLayerFt(covers[i], levels[i])
		if !ok {
			continue
		}

		if best == nil || ft < *best {
			v := ft
			best = &v
		}
	}

	return best
}

func ceilingLayerFt(cover string, level *float64) (int, bool) {
	if level == nil || math.IsNaN(*level) || math.IsInf(*level, 0) {
		return 0, false
	}

	switch strings.ToUpper(strings.TrimSpace(cover)) {
	case coverBKN, coverOVC, coverVV:
	default:
		return 0, false
	}

	if *level < 0 || *level > weatherCeilingMaxFt {
		return 0, false
	}

	return int(math.Round(*level)), true
}

func clampVisibility(raw *float64) *float64 {
	if raw == nil || math.IsNaN(*raw) || math.IsInf(*raw, 0) || *raw < 0 {
		return nil
	}

	v := *raw
	if v > weatherVisibilityCapSM {
		v = weatherVisibilityCapSM
	}

	return &v
}

func maxWindKt(sknt, gust *float64) *float64 {
	var best *float64

	for _, raw := range []*float64{sknt, gust} {
		if raw == nil || math.IsNaN(*raw) || math.IsInf(*raw, 0) || *raw < 0 || *raw > weatherWindMaxKt {
			continue
		}

		if best == nil || *raw > *best {
			v := *raw
			best = &v
		}
	}

	return best
}

func scanWxCodes(raw string) (thunder, precip bool) {
	for _, tok := range strings.Fields(raw) {
		tok = strings.TrimLeft(strings.ToUpper(tok), "+-")
		if tok == "" {
			continue
		}

		if strings.Contains(tok, "TS") {
			thunder = true
		}

		if tokenPrecip(tok) {
			precip = true
		}
	}

	return thunder, precip
}

func tokenPrecip(tok string) bool {
	if strings.Contains(tok, "FZ") {
		return true
	}

	for _, needle := range precipPhenomena {
		if strings.Contains(tok, needle) {
			return true
		}
	}

	return false
}

type wxColumnIndex struct {
	vsby    int
	sknt    int
	gust    int
	wxcodes int
	skyc    [3]int
	skyl    [3]int
}

func withCategoryColumns(columns []string, rows [][]string) ([]string, [][]string, error) {
	idx, err := wxColumns(columns)
	if err != nil {
		return nil, nil, err
	}

	outCols := make([]string, 0, len(columns)+2)
	outCols = append(outCols, columns...)
	outCols = append(outCols, colCeilingFt, colCategory)

	outRows := make([][]string, len(rows))

	for i, row := range rows {
		if len(row) != len(columns) {
			return nil, nil, fmt.Errorf("row %d width %d does not match columns %d", i+1, len(row), len(columns))
		}

		fields, err := metarFields(row, idx)
		if err != nil {
			return nil, nil, fmt.Errorf("row %d: %w", i+1, err)
		}

		ceiling, category := ClassifyMetar(fields)

		out := make([]string, 0, len(row)+2)
		out = append(out, row...)
		out = append(out, formatCeiling(ceiling), category)
		outRows[i] = out
	}

	return outCols, outRows, nil
}

func wxColumns(columns []string) (wxColumnIndex, error) {
	idx := wxColumnIndex{
		vsby: -1, sknt: -1, gust: -1, wxcodes: -1,
		skyc: [3]int{-1, -1, -1},
		skyl: [3]int{-1, -1, -1},
	}

	for i, col := range columns {
		switch col {
		case colVsby:
			idx.vsby = i
		case colSknt:
			idx.sknt = i
		case colGust:
			idx.gust = i
		case colWxcodes:
			idx.wxcodes = i
		case colSkyc1:
			idx.skyc[0] = i
		case colSkyc2:
			idx.skyc[1] = i
		case colSkyc3:
			idx.skyc[2] = i
		case colSkyl1:
			idx.skyl[0] = i
		case colSkyl2:
			idx.skyl[1] = i
		case colSkyl3:
			idx.skyl[2] = i
		}
	}

	missing := make([]string, 0)
	require := []struct {
		name string
		at   int
	}{
		{colVsby, idx.vsby},
		{colSknt, idx.sknt},
		{colGust, idx.gust},
		{colWxcodes, idx.wxcodes},
		{colSkyc1, idx.skyc[0]},
		{colSkyc2, idx.skyc[1]},
		{colSkyc3, idx.skyc[2]},
		{colSkyl1, idx.skyl[0]},
		{colSkyl2, idx.skyl[1]},
		{colSkyl3, idx.skyl[2]},
	}

	for _, col := range require {
		if col.at < 0 {
			missing = append(missing, col.name)
		}
	}

	if len(missing) > 0 {
		return wxColumnIndex{}, fmt.Errorf("weather category columns missing: %s", strings.Join(missing, ", "))
	}

	return idx, nil
}

func metarFields(row []string, idx wxColumnIndex) (MetarFields, error) {
	vsby, err := parseOptionalFloat(row[idx.vsby])
	if err != nil {
		return MetarFields{}, fmt.Errorf("vsby: %w", err)
	}

	sknt, err := parseOptionalFloat(row[idx.sknt])
	if err != nil {
		return MetarFields{}, fmt.Errorf("sknt: %w", err)
	}

	gust, err := parseOptionalFloat(row[idx.gust])
	if err != nil {
		return MetarFields{}, fmt.Errorf("gust: %w", err)
	}

	var fields MetarFields

	fields.Vsby = vsby
	fields.Sknt = sknt
	fields.Gust = gust
	fields.WxCodes = row[idx.wxcodes]

	for i := range idx.skyc {
		fields.SkyCover[i] = row[idx.skyc[i]]

		level, err := parseOptionalFloat(row[idx.skyl[i]])
		if err != nil {
			return MetarFields{}, fmt.Errorf("skyl%d: %w", i+1, err)
		}

		fields.SkyLevel[i] = level
	}

	return fields, nil
}

func parseOptionalFloat(raw string) (*float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", raw, err)
	}

	if math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, nil
	}

	return &v, nil
}

func formatCeiling(ceiling *int) string {
	if ceiling == nil {
		return ""
	}

	return strconv.Itoa(*ceiling)
}
