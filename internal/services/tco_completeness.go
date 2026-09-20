package services

import (
	"math"
)

// completenessInputs gathers the ratios used by the completeness score.
type completenessInputs struct {
	kwhAdded, kwhPriced float64
	highwayDrives       int
	unqualifiedDrives   int
	trackedKm, basisKm  float64
	insurancePresent    bool
	acquisitionComplete bool
	pricedEntries       int
	unconvertedEntries  int
	drivesWithOdometer  int
	odometerAnomalies   int
	ice                 bool // Combustion vehicle: energy and distance come from fuel fill-ups
	iceFillUps          int
}

func ratio(part, total float64) float64 {
	if total <= 0 {
		return 1
	}
	return math.Max(0, math.Min(1, part/total))
}

// completion is the share of items without a problem; with no item at all there is nothing missing.
func completion(problems, total float64) float64 {
	if total <= 0 {
		return 1
	}
	return 1 - ratio(problems, total)
}

func boolScore(ok bool) float64 {
	if ok {
		return 1
	}
	return 0
}

// completenessScore weights how much of the TCO rests on complete data.
func completenessScore(in completenessInputs) (int, []CompletenessDimension) {
	distance := 0.0
	if in.basisKm > 0 {
		distance = ratio(in.trackedKm, in.basisKm)
	}
	energyLabel, energyScore := "Charges with a cost (kWh)", ratio(in.kwhPriced, in.kwhAdded)
	distanceLabel := "Kilometres covered by drives"
	if in.ice {
		energyLabel, energyScore = "Fuel fill-ups recorded", boolScore(in.iceFillUps > 0)
		distanceLabel = "Kilometres covered by readings and fill-ups"
	}
	dims := []struct {
		key, label string
		weight     float64
		score      float64
	}{
		{"energy", energyLabel, 0.30, energyScore},
		{"distance", distanceLabel, 0.20, distance},
		{"tolls", "Motorway drives qualified", 0.15, completion(float64(in.unqualifiedDrives), float64(in.highwayDrives))},
		{"insurance", "Insurance entered", 0.10, boolScore(in.insurancePresent)},
		{"acquisition", "Acquisition and depreciation entered", 0.10, boolScore(in.acquisitionComplete)},
		{"odometer", "Odometer continuity", 0.10, completion(float64(in.odometerAnomalies), float64(in.drivesWithOdometer))},
		{"currency", "Expenses converted to euros", 0.05, completion(float64(in.unconvertedEntries), float64(in.pricedEntries+in.unconvertedEntries))},
	}
	var total float64
	out := make([]CompletenessDimension, 0, len(dims))
	for _, d := range dims {
		total += d.weight * d.score
		out = append(out, CompletenessDimension{Key: d.key, Label: d.label, ScorePct: int(math.Round(d.score * 100)), Weight: d.weight})
	}
	return int(math.Round(total * 100)), out
}
