package services

import (
	"sort"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// AnnotatedFuelLog is a fill-up with the figures measured on the segment it closes.
// Consumption is only known for a full tank that follows another full tank, with litres
// recorded on every fill in between (the "full-to-full" method).
type AnnotatedFuelLog struct {
	models.FuelLog
	SegmentKm       *float64 `json:"segment_km,omitempty"`
	ConsumptionL100 *float64 `json:"consumption_l_100km,omitempty"`
	CostPerKm       *float64 `json:"cost_per_km,omitempty"`
}

// FuelStats summarizes the fill-ups of a vehicle.
type FuelStats struct {
	TotalCost        money.Cents        `json:"total_cost"`
	TotalLiters      float64            `json:"total_liters"` // Litres of the fill-ups where they are recorded
	AvgPricePerLiter float64            `json:"avg_price_per_liter"`
	ConsumptionL100  *float64           `json:"consumption_l_100km,omitempty"` // Over all measurable segments
	MeasuredKm       float64            `json:"measured_km"`
	MeasurableCount  int                `json:"measurable_segments"`
	UnmeasurableCnt  int                `json:"unmeasurable_segments"` // Full-to-full segments lacking litres or distance
	FillUps          int                `json:"fill_ups"`
	Logs             []AnnotatedFuelLog `json:"logs"`
}

// ComputeFuelStats is a pure function over the fill-ups of one vehicle (any order).
func ComputeFuelStats(logs []models.FuelLog) FuelStats {
	sorted := append([]models.FuelLog(nil), logs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Odometer != sorted[j].Odometer {
			return sorted[i].Odometer < sorted[j].Odometer
		}
		return sorted[i].Date.Before(sorted[j].Date)
	})

	st := FuelStats{FillUps: len(sorted), Logs: make([]AnnotatedFuelLog, len(sorted))}
	var pricedCost, pricedLiters float64
	for i, f := range sorted {
		st.TotalCost += f.Amount
		st.Logs[i] = AnnotatedFuelLog{FuelLog: f}
		if f.Liters != nil && *f.Liters > 0 {
			st.TotalLiters += *f.Liters
			pricedCost += f.Amount.Float()
			pricedLiters += *f.Liters
		}
	}
	if pricedLiters > 0 {
		st.AvgPricePerLiter = round3(pricedCost / pricedLiters)
	}

	// Full-to-full segments: the litres added at a full tank refill what was used since the previous full tank.
	anchor := -1
	var segLiters, segCost float64
	segComplete := true
	var measuredLiters float64
	for i, f := range sorted {
		if anchor >= 0 {
			segCost += f.Amount.Float()
			if f.Liters == nil || *f.Liters <= 0 {
				segComplete = false
			} else {
				segLiters += *f.Liters
			}
		}
		if !f.IsFullTank {
			continue
		}
		if anchor >= 0 {
			km := f.Odometer - sorted[anchor].Odometer
			if segComplete && km > 0 {
				l100 := round3(segLiters / km * 100)
				cpk := round3(segCost / km)
				st.Logs[i].SegmentKm = &km
				st.Logs[i].ConsumptionL100 = &l100
				st.Logs[i].CostPerKm = &cpk
				st.MeasurableCount++
				st.MeasuredKm += km
				measuredLiters += segLiters
			} else {
				st.UnmeasurableCnt++
			}
		}
		anchor, segLiters, segCost, segComplete = i, 0, 0, true
	}
	if st.MeasuredKm > 0 {
		v := round3(measuredLiters / st.MeasuredKm * 100)
		st.ConsumptionL100 = &v
	}
	st.TotalLiters = round1(st.TotalLiters)
	return st
}
