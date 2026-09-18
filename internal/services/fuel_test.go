package services

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

func fill(odo, amount float64, liters *float64, full bool) models.FuelLog {
	return models.FuelLog{
		Date:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(odo/100)),
		Odometer:   odo,
		Amount:     eur(amount),
		Liters:     liters,
		IsFullTank: full,
	}
}

func lit(v float64) *float64 { return &v }

func TestComputeFuelStatsFullToFull(t *testing.T) {
	st := ComputeFuelStats([]models.FuelLog{
		fill(10000, 80, lit(50), true), // anchor, no segment
		fill(10500, 40, lit(25), true), // 500 km, 25 L -> 5 L/100
		fill(11100, 60, lit(36), true), // 600 km, 36 L -> 6 L/100
	})
	if st.FillUps != 3 || st.TotalCost != eur(180) || st.TotalLiters != 111 {
		t.Fatalf("totals: %+v", st)
	}
	if st.ConsumptionL100 == nil || *st.ConsumptionL100 != 5.545 { // 61 L / 1100 km
		t.Fatalf("consumption = %v, want 5.545", st.ConsumptionL100)
	}
	if st.MeasurableCount != 2 || st.UnmeasurableCnt != 0 || st.MeasuredKm != 1100 {
		t.Errorf("segments: %+v", st)
	}
	if st.Logs[0].ConsumptionL100 != nil {
		t.Errorf("first fill-up has no segment")
	}
	if got := st.Logs[1].ConsumptionL100; got == nil || *got != 5 {
		t.Errorf("second consumption = %v, want 5", got)
	}
	if got := st.Logs[2].CostPerKm; got == nil || *got != 0.1 { // 60 EUR / 600 km
		t.Errorf("third cost/km = %v, want 0.1", got)
	}
	if st.AvgPricePerLiter != 1.622 { // 180 EUR / 111 L
		t.Errorf("avg price = %v, want 1.622", st.AvgPricePerLiter)
	}
}

func TestComputeFuelStatsPartialFillsAccumulate(t *testing.T) {
	st := ComputeFuelStats([]models.FuelLog{
		fill(1000, 60, lit(40), true),
		fill(1200, 20, lit(12), false), // partial, folded into the next full segment
		fill(1500, 40, lit(23), true),  // 500 km, 35 L -> 7 L/100, cost 60 EUR
	})
	if st.MeasurableCount != 1 || st.ConsumptionL100 == nil || *st.ConsumptionL100 != 7 {
		t.Fatalf("stats: %+v", st)
	}
	if st.Logs[1].ConsumptionL100 != nil {
		t.Errorf("a partial fill has no consumption of its own")
	}
	if got := st.Logs[2].CostPerKm; got == nil || *got != 0.12 {
		t.Errorf("cost/km = %v, want 0.12", got)
	}
}

func TestComputeFuelStatsUnmeasurable(t *testing.T) {
	tests := []struct {
		name string
		logs []models.FuelLog
		want int // unmeasurable segments
	}{
		{"missing liters on the closing fill", []models.FuelLog{fill(1000, 50, lit(30), true), fill(1500, 40, nil, true)}, 1},
		{"missing liters on a partial fill", []models.FuelLog{fill(1000, 50, lit(30), true), fill(1200, 20, nil, false), fill(1500, 40, lit(20), true)}, 1},
		{"no distance between fills", []models.FuelLog{fill(1000, 50, lit(30), true), fill(1000, 40, lit(20), true)}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := ComputeFuelStats(tt.logs)
			if st.UnmeasurableCnt != tt.want || st.MeasurableCount != 0 || st.ConsumptionL100 != nil {
				t.Errorf("stats: %+v", st)
			}
		})
	}
}

func TestComputeFuelStatsEdges(t *testing.T) {
	if st := ComputeFuelStats(nil); st.FillUps != 0 || st.ConsumptionL100 != nil || len(st.Logs) != 0 || st.AvgPricePerLiter != 0 {
		t.Errorf("empty: %+v", st)
	}
	// A single full tank, or partial fills only, never yields a consumption.
	if st := ComputeFuelStats([]models.FuelLog{fill(1000, 50, lit(30), true)}); st.ConsumptionL100 != nil || st.MeasurableCount != 0 {
		t.Errorf("single: %+v", st)
	}
	if st := ComputeFuelStats([]models.FuelLog{fill(1000, 50, lit(30), false), fill(1500, 50, lit(30), false)}); st.ConsumptionL100 != nil {
		t.Errorf("partial only: %+v", st)
	}
	// Input order does not matter and the input slice is not modified.
	in := []models.FuelLog{fill(1500, 40, lit(25), true), fill(1000, 60, lit(40), true)}
	st := ComputeFuelStats(in)
	if in[0].Odometer != 1500 || st.Logs[0].Odometer != 1000 || st.ConsumptionL100 == nil || *st.ConsumptionL100 != 5 {
		t.Errorf("ordering: %+v", st)
	}
	// Amount-only fill-ups count in the cost but not in the price per litre.
	if st := ComputeFuelStats([]models.FuelLog{fill(1000, 50, nil, true), fill(1500, 40, lit(20), true)}); st.TotalCost != eur(90) || st.AvgPricePerLiter != 2 {
		t.Errorf("mixed: %+v", st)
	}
}
