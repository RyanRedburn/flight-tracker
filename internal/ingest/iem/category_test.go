package iem

import (
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

func TestClassifyMetarPriority(t *testing.T) {
	t.Parallel()

	ceil1500 := 1500
	ceil900 := 900
	ceil400 := 400
	ceil300 := 300
	ceil1000 := 1000
	ceil3000 := 3000
	ceil3001 := 3001

	tests := []struct {
		name     string
		fields   MetarFields
		ceiling  *int
		category string
	}{
		{
			name: "thunder beats lifr visibility",
			fields: MetarFields{
				Vsby:     fptr(0.5),
				Sknt:     fptr(10),
				SkyCover: [3]string{coverOVC, "", ""},
				SkyLevel: [3]*float64{fptr(200), nil, nil},
				WxCodes:  "TSRA",
			},
			ceiling:  iptr(200),
			category: model.WeatherCategoryThunder,
		},
		{
			name: "vcts and intensity prefixes are thunder",
			fields: MetarFields{
				Vsby:    fptr(10),
				Sknt:    fptr(8),
				WxCodes: "+TSRA VCTS -TS",
			},
			category: model.WeatherCategoryThunder,
		},
		{
			name: "thunder with missing vis ceiling and wind",
			fields: MetarFields{
				WxCodes: "TS",
			},
			category: model.WeatherCategoryThunder,
		},
		{
			name: "lifr ceiling under 500",
			fields: MetarFields{
				Vsby:     fptr(10),
				Sknt:     fptr(5),
				SkyCover: [3]string{coverBKN, "", ""},
				SkyLevel: [3]*float64{fptr(400), nil, nil},
			},
			ceiling:  &ceil400,
			category: model.WeatherCategoryLIFR,
		},
		{
			name: "lifr visibility under 1",
			fields: MetarFields{
				Vsby: fptr(0.9),
				Sknt: fptr(5),
			},
			category: model.WeatherCategoryLIFR,
		},
		{
			name: "ceiling exactly 500 is ifr not lifr",
			fields: MetarFields{
				Vsby:     fptr(10),
				Sknt:     fptr(5),
				SkyCover: [3]string{coverOVC, "", ""},
				SkyLevel: [3]*float64{fptr(500), nil, nil},
			},
			ceiling:  iptr(500),
			category: model.WeatherCategoryIFR,
		},
		{
			name: "visibility exactly 1 is ifr not lifr",
			fields: MetarFields{
				Vsby: fptr(1),
				Sknt: fptr(5),
			},
			category: model.WeatherCategoryIFR,
		},
		{
			name: "ifr ceiling under 1000",
			fields: MetarFields{
				Vsby:     fptr(10),
				Sknt:     fptr(5),
				SkyCover: [3]string{coverFEW, coverBKN, coverOVC},
				SkyLevel: [3]*float64{fptr(200), fptr(900), fptr(8000)},
			},
			ceiling:  &ceil900,
			category: model.WeatherCategoryIFR,
		},
		{
			name: "visibility exactly 3 is mvfr not ifr",
			fields: MetarFields{
				Vsby: fptr(3),
				Sknt: fptr(5),
			},
			category: model.WeatherCategoryMVFR,
		},
		{
			name: "ceiling exactly 1000 is mvfr",
			fields: MetarFields{
				Vsby:     fptr(10),
				Sknt:     fptr(5),
				SkyCover: [3]string{coverOVC, "", ""},
				SkyLevel: [3]*float64{fptr(1000), nil, nil},
			},
			ceiling:  &ceil1000,
			category: model.WeatherCategoryMVFR,
		},
		{
			name: "mvfr ceiling at 3000",
			fields: MetarFields{
				Vsby:     fptr(10),
				Sknt:     fptr(5),
				SkyCover: [3]string{coverBKN, "", ""},
				SkyLevel: [3]*float64{fptr(3000), nil, nil},
			},
			ceiling:  &ceil3000,
			category: model.WeatherCategoryMVFR,
		},
		{
			name: "visibility exactly 5 is mvfr",
			fields: MetarFields{
				Vsby: fptr(5),
				Sknt: fptr(5),
			},
			category: model.WeatherCategoryMVFR,
		},
		{
			name: "vv counts and lowest of skyc1-3 wins",
			fields: MetarFields{
				Vsby:     fptr(10),
				Sknt:     fptr(4),
				SkyCover: [3]string{coverSCT, coverVV, coverOVC},
				SkyLevel: [3]*float64{fptr(100), fptr(300), fptr(5000)},
			},
			ceiling:  &ceil300,
			category: model.WeatherCategoryLIFR,
		},
		{
			name: "few and sct are not a ceiling",
			fields: MetarFields{
				Vsby:     fptr(10),
				Sknt:     fptr(4),
				SkyCover: [3]string{coverFEW, coverSCT, ""},
				SkyLevel: [3]*float64{fptr(200), fptr(400), nil},
			},
			category: model.WeatherCategoryVFRFair,
		},
		{
			name: "windy uses greater of gust and sknt",
			fields: MetarFields{
				Vsby: fptr(10),
				Sknt: fptr(10),
				Gust: fptr(30),
			},
			category: model.WeatherCategoryWindy,
		},
		{
			name: "sknt at 25 is windy when gust missing",
			fields: MetarFields{
				Vsby: fptr(10),
				Sknt: fptr(25),
			},
			category: model.WeatherCategoryWindy,
		},
		{
			name: "gust missing and sknt 24 is not windy",
			fields: MetarFields{
				Vsby: fptr(10),
				Sknt: fptr(24),
			},
			category: model.WeatherCategoryVFRFair,
		},
		{
			name: "mvfr beats windy",
			fields: MetarFields{
				Vsby: fptr(4),
				Sknt: fptr(40),
			},
			category: model.WeatherCategoryMVFR,
		},
		{
			name: "precip rain when otherwise vfr",
			fields: MetarFields{
				Vsby:     fptr(10),
				Sknt:     fptr(8),
				SkyCover: [3]string{coverOVC, "", ""},
				SkyLevel: [3]*float64{fptr(5000), nil, nil},
				WxCodes:  "-RA",
			},
			ceiling:  iptr(5000),
			category: model.WeatherCategoryPrecip,
		},
		{
			name: "fzra and fzfg are precip",
			fields: MetarFields{
				Vsby:    fptr(10),
				Sknt:    fptr(8),
				WxCodes: "FZFG",
			},
			category: model.WeatherCategoryPrecip,
		},
		{
			name: "blowing snow is precip",
			fields: MetarFields{
				Vsby:    fptr(10),
				Sknt:    fptr(8),
				WxCodes: "BLSN",
			},
			category: model.WeatherCategoryPrecip,
		},
		{
			name: "br only with visibility above 5 is vfr fair",
			fields: MetarFields{
				Vsby:     fptr(6),
				Sknt:     fptr(8),
				SkyCover: [3]string{coverBKN, "", ""},
				SkyLevel: [3]*float64{fptr(4000), nil, nil},
				WxCodes:  "BR",
			},
			ceiling:  iptr(4000),
			category: model.WeatherCategoryVFRFair,
		},
		{
			name: "br only with low visibility is mvfr not precip",
			fields: MetarFields{
				Vsby:    fptr(4),
				Sknt:    fptr(8),
				WxCodes: "BR",
			},
			category: model.WeatherCategoryMVFR,
		},
		{
			name: "empty wxcodes can be fair",
			fields: MetarFields{
				Vsby: fptr(10),
				Sknt: fptr(8),
			},
			category: model.WeatherCategoryVFRFair,
		},
		{
			name: "precip when vis ceiling and wind missing",
			fields: MetarFields{
				WxCodes: "RA",
			},
			category: model.WeatherCategoryPrecip,
		},
		{
			name: "unknown when vis ceiling and wind missing and no phenomena",
			fields: MetarFields{
				WxCodes: "BR",
			},
			category: model.WeatherCategoryUnknown,
		},
		{
			name:     "unknown when observation is empty",
			fields:   MetarFields{},
			category: model.WeatherCategoryUnknown,
		},
		{
			name: "visibility clamp does not invent mvfr",
			fields: MetarFields{
				Vsby: fptr(20),
				Sknt: fptr(8),
			},
			category: model.WeatherCategoryVFRFair,
		},
		{
			name: "absurd wind is ignored",
			fields: MetarFields{
				Vsby: fptr(10),
				Sknt: fptr(250),
			},
			category: model.WeatherCategoryVFRFair,
		},
		{
			name: "absurd ceiling is ignored",
			fields: MetarFields{
				Vsby:     fptr(10),
				Sknt:     fptr(8),
				SkyCover: [3]string{coverOVC, "", ""},
				SkyLevel: [3]*float64{fptr(100001), nil, nil},
			},
			category: model.WeatherCategoryVFRFair,
		},
		{
			name: "negative visibility is missing",
			fields: MetarFields{
				Vsby: fptr(-1),
				Sknt: fptr(8),
			},
			category: model.WeatherCategoryVFRFair,
		},
		{
			name: "ceiling just above mvfr is vfr",
			fields: MetarFields{
				Vsby:     fptr(10),
				Sknt:     fptr(8),
				SkyCover: [3]string{"bkn", "", ""},
				SkyLevel: [3]*float64{fptr(3001), nil, nil},
			},
			ceiling:  &ceil3001,
			category: model.WeatherCategoryVFRFair,
		},
		{
			name: "mvfr ceiling from ovc 1500 beats snow precip",
			fields: MetarFields{
				Vsby:     fptr(10),
				Sknt:     fptr(11),
				SkyCover: [3]string{coverOVC, "", ""},
				SkyLevel: [3]*float64{fptr(1500), nil, nil},
				WxCodes:  "-SN",
			},
			ceiling:  &ceil1500,
			category: model.WeatherCategoryMVFR,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotCeiling, gotCategory := ClassifyMetar(tt.fields)
			if gotCategory != tt.category {
				t.Fatalf("category = %s, want %s", gotCategory, tt.category)
			}

			if !sameCeiling(gotCeiling, tt.ceiling) {
				t.Fatalf("ceiling = %v, want %v", formatCeiling(gotCeiling), formatCeiling(tt.ceiling))
			}
		})
	}
}

func TestWithCategoryColumns(t *testing.T) {
	t.Parallel()

	columns := []string{colVsby, colSknt, colGust, colSkyc1, colSkyl1, colSkyc2, colSkyl2, colSkyc3, colSkyl3, colWxcodes}
	rows := [][]string{
		{"10.00", "11.00", "", coverOVC, "1500.00", "", "", "", "", "-SN"},
	}

	outCols, outRows, err := withCategoryColumns(columns, rows)
	if err != nil {
		t.Fatalf("withCategoryColumns() error = %v", err)
	}

	if outCols[len(outCols)-2] != colCeilingFt || outCols[len(outCols)-1] != colCategory {
		t.Fatalf("columns = %v", outCols)
	}

	if outRows[0][len(outRows[0])-2] != "1500" || outRows[0][len(outRows[0])-1] != model.WeatherCategoryMVFR {
		t.Fatalf("classified = %v", outRows[0][len(outRows[0])-2:])
	}
}

func fptr(v float64) *float64 {
	return &v
}

func iptr(v int) *int {
	return &v
}

func sameCeiling(got, want *int) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}

	return *got == *want
}
