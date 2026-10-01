package store

import (
	"math"
	"sort"
	"testing"

	"github.com/RyanRedburn/flight-tracker/internal/model"
)

func TestConnectionRecommendation(t *testing.T) {
	// Arrived delays only. The last early flight is a late outlier past p90.
	earlyP90 := percentileCont(0.9, []float64{-40, -30, -25, -20, -18, -15, -12, -10, -8, -6, -4, 200})
	lateP90 := percentileCont(0.9, []float64{-20, -10, 30, 40, 50, 60, 80, 100, 140, 180, 180, 200})

	if earlyP90 >= 0 {
		t.Fatalf("early-heavy p90 = %v, want negative", earlyP90)
	}

	if lateP90 != 180 {
		t.Fatalf("late-heavy p90 = %v, want 180", lateP90)
	}

	tests := []struct {
		name         string
		insufficient bool
		p90          *float64
		mct          *AirportConnectionMCT
		want         model.ConnectionGuidance
		floors       [4]bool
	}{
		{
			name:   "early-heavy",
			p90:    &earlyP90,
			want:   minutes(45, 60, 120, 120),
			floors: allFloors(),
		},
		{
			name:   "late-heavy",
			p90:    &lateP90,
			want:   minutes(225, 240, 300, 300),
			floors: allFloors(),
		},
		{
			name: "cancel-only",
			mct:  allMCT(90),
		},
		{
			name:         "insufficient sample",
			insufficient: true,
			p90:          &lateP90,
			mct:          allMCT(90),
		},
		{
			name: "mct higher than delay",
			p90:  floatPtr(20),
			mct: &AirportConnectionMCT{
				DomesticToDomestic:           intPtr(75),
				DomesticToInternational:      intPtr(90),
				InternationalToDomestic:      intPtr(150),
				InternationalToInternational: intPtr(180),
			},
			want: minutes(95, 110, 170, 200),
		},
		{
			name:   "mct missing uses static floors",
			p90:    floatPtr(50),
			want:   minutes(95, 110, 170, 170),
			floors: allFloors(),
		},
		{
			name:   "mct row with null buckets uses static floors",
			p90:    floatPtr(50),
			mct:    &AirportConnectionMCT{},
			want:   minutes(95, 110, 170, 170),
			floors: allFloors(),
		},
		{
			name: "mct present for only some buckets",
			p90:  floatPtr(50),
			mct: &AirportConnectionMCT{
				DomesticToDomestic:           intPtr(80),
				InternationalToDomestic:      intPtr(30),
				InternationalToInternational: intPtr(200),
			},
			want:   minutes(130, 110, 80, 250),
			floors: [4]bool{false, true, false, false},
		},
		{
			name: "rounds half away from zero",
			p90:  floatPtr(44.5),
			mct: &AirportConnectionMCT{
				DomesticToDomestic: intPtr(40),
			},
			want:   minutes(85, 105, 165, 165),
			floors: [4]bool{false, true, true, true},
		},
		{
			name: "delay plus floor",
			p90:  floatPtr(30),
			mct: &AirportConnectionMCT{
				DomesticToDomestic: intPtr(45),
			},
			want:   minutes(75, 90, 150, 150),
			floors: [4]bool{false, true, true, true},
		},
		{
			name: "airport minute equal to static floor is not floor-only",
			p90:  floatPtr(0),
			mct: &AirportConnectionMCT{
				DomesticToDomestic: intPtr(connectionFloorDomesticToDomestic),
			},
			want:   minutes(connectionFloorDomesticToDomestic, connectionFloorDomesticToInternational, connectionFloorInternationalToDomestic, connectionFloorInternationalToInternational),
			floors: [4]bool{false, true, true, true},
		},
		{
			name: "zero mct and early delay is zero",
			p90:  floatPtr(-3),
			mct:  allMCT(0),
			want: minutes(0, 0, 0, 0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ConnectionRecommendation(tt.insufficient, tt.p90, tt.mct)
			assertConnectionGuidance(t, got, withFloors(tt.want, tt.floors))
		})
	}
}

func assertConnectionGuidance(t *testing.T, got, want model.ConnectionGuidance) {
	t.Helper()

	assertConnectionMinutes(t, "domestic_to_domestic", got.DomesticToDomestic, want.DomesticToDomestic)
	assertConnectionMinutes(t, "domestic_to_international", got.DomesticToInternational, want.DomesticToInternational)
	assertConnectionMinutes(t, "international_to_domestic", got.InternationalToDomestic, want.InternationalToDomestic)
	assertConnectionMinutes(t, "international_to_international", got.InternationalToInternational, want.InternationalToInternational)
}

func assertConnectionMinutes(t *testing.T, name string, got, want *model.ConnectionMinutes) {
	t.Helper()

	if want == nil && got == nil {
		return
	}

	if want == nil || got == nil || *got != *want {
		t.Errorf("%s = %+v, want %+v", name, got, want)
	}
}

func minutes(dd, di, id, ii int) model.ConnectionGuidance {
	return model.ConnectionGuidanceFromRecommended(dd, di, id, ii)
}

func withFloors(g model.ConnectionGuidance, floors [4]bool) model.ConnectionGuidance {
	buckets := []*model.ConnectionMinutes{
		g.DomesticToDomestic,
		g.DomesticToInternational,
		g.InternationalToDomestic,
		g.InternationalToInternational,
	}

	for i, bucket := range buckets {
		if bucket != nil {
			bucket.FloorOnly = floors[i]
		}
	}

	return g
}

func allFloors() [4]bool {
	return [4]bool{true, true, true, true}
}

func floatPtr(v float64) *float64 {
	return &v
}

// percentileCont matches PostgreSQL percentile_cont: RN = 1 + p*(N-1), interpolating.
func percentileCont(p float64, values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	n := len(sorted)
	if n == 0 {
		return 0
	}

	if n == 1 {
		return sorted[0]
	}

	rn := 1 + p*float64(n-1)
	lo := int(math.Floor(rn))
	hi := int(math.Ceil(rn))

	if lo == hi {
		return sorted[lo-1]
	}

	return sorted[lo-1] + (rn-float64(lo))*(sorted[hi-1]-sorted[lo-1])
}

func intPtr(v int) *int {
	return &v
}

func allMCT(v int) *AirportConnectionMCT {
	return &AirportConnectionMCT{
		DomesticToDomestic:           intPtr(v),
		DomesticToInternational:      intPtr(v),
		InternationalToDomestic:      intPtr(v),
		InternationalToInternational: intPtr(v),
	}
}
