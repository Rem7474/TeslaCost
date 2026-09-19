package services

import "sort"

const (
	tempBinWidthC = 5

	// Short hops are dominated by warm-up and stops, whatever the weather.
	minDriveKmForTemperature = 5.0
	// A bin with less distance than this is too noisy to be shown.
	minBinDistanceKm = 50.0

	// The winter effect compares the drives below this temperature with the mild ones.
	coldBelowC          = 5
	mildFromC           = 15
	mildToC             = 25
	minEffectDistanceKm = 100.0
)

// TemperatureBin is the consumption of the drives whose average outside temperature falls in [MinC, MaxC).
type TemperatureBin struct {
	MinC                int     `json:"min_c"`
	MaxC                int     `json:"max_c"`
	Drives              int     `json:"drives"`
	DistanceKm          float64 `json:"distance_km"`
	ConsumptionKwh100km float64 `json:"consumption_kwh_100km"`
}

// TemperatureEffect quantifies the extra consumption of cold weather against mild weather.
type TemperatureEffect struct {
	ColdConsumptionKwh100km *float64 `json:"cold_consumption_kwh_100km,omitempty"` // Below 5 °C
	MildConsumptionKwh100km *float64 `json:"mild_consumption_kwh_100km,omitempty"` // 15 to 25 °C
	ExtraPercent            *float64 `json:"extra_percent,omitempty"`
	// ExtraCostPer100km is the cost of that extra energy at the average price paid per kWh.
	ExtraCostPer100km *float64 `json:"extra_cost_per_100km,omitempty"`
}

// tempBinRaw is the drives of a temperature bin, aggregated by the database.
type tempBinRaw struct {
	MinC       int
	Drives     int
	DistanceKm float64
	Kwh        float64
}

// computeTemperature derives the displayed bins and the winter effect from the aggregated drives.
func computeTemperature(raw []tempBinRaw, pricePerKwh *float64) ([]TemperatureBin, TemperatureEffect) {
	sorted := append([]tempBinRaw(nil), raw...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].MinC < sorted[j].MinC })

	bins := []TemperatureBin{}
	var coldKm, coldKwh, mildKm, mildKwh float64
	for _, b := range sorted {
		if b.DistanceKm <= 0 || b.Kwh <= 0 {
			continue
		}
		if b.DistanceKm >= minBinDistanceKm {
			bins = append(bins, TemperatureBin{
				MinC:                b.MinC,
				MaxC:                b.MinC + tempBinWidthC,
				Drives:              b.Drives,
				DistanceKm:          round1(b.DistanceKm),
				ConsumptionKwh100km: round1(b.Kwh / b.DistanceKm * 100),
			})
		}
		// The effect uses every bin, however small: the groups are pooled before the distance threshold applies.
		if b.MinC+tempBinWidthC <= coldBelowC {
			coldKm += b.DistanceKm
			coldKwh += b.Kwh
		}
		if b.MinC >= mildFromC && b.MinC+tempBinWidthC <= mildToC {
			mildKm += b.DistanceKm
			mildKwh += b.Kwh
		}
	}

	var effect TemperatureEffect
	if coldKm >= minEffectDistanceKm && mildKm >= minEffectDistanceKm {
		cold := coldKwh / coldKm * 100
		mild := mildKwh / mildKm * 100
		effect.ColdConsumptionKwh100km = ratioPtr(cold, 1, 1)
		effect.MildConsumptionKwh100km = ratioPtr(mild, 1, 1)
		if cold > mild {
			effect.ExtraPercent = ratioPtr((cold-mild)*100, mild, 1)
			if pricePerKwh != nil {
				extraKwh := cold - mild
				effect.ExtraCostPer100km = ratioPtr(extraKwh*(*pricePerKwh), 1, 2)
			}
		}
	}
	return bins, effect
}
