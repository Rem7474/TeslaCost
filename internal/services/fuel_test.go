package services

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

// fillDay is the date of the day-th day of the test calendar.
func fillDay(day int) time.Time {
	return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, day)
}

// fill builds a fill-up whose date follows its odometer (100 km per day) so that both orders agree.
func fill(odo, amount float64, liters *float64, full bool) models.FuelLog {
	return models.FuelLog{
		Date:       fillDay(int(odo / 100)),
		Odometer:   &odo,
		Amount:     eur(amount),
		Liters:     liters,
		IsFullTank: full,
	}
}

// fillNoKm builds a fill-up entered without mileage on the given day.
func fillNoKm(day int, amount float64, liters *float64, full bool) models.FuelLog {
	return models.FuelLog{Date: fillDay(day), Amount: eur(amount), Liters: liters, IsFullTank: full}
}

func lit(v float64) *float64 { return &v }

func TestComputeFuelStatsFullToFull(t *testing.T) {
	st := ComputeFuelStats([]models.FuelLog{
		fill(10000, 80, lit(50), true), // anchor, no segment
		fill(10500, 40, lit(25), true), // 500 km, 25 L -> 5 L/100
		fill(11100, 60, lit(36), true), // 600 km, 36 L -> 6 L/100
	}, nil)
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
	}, nil)
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
			st := ComputeFuelStats(tt.logs, nil)
			if st.UnmeasurableCnt != tt.want || st.MeasurableCount != 0 || st.ConsumptionL100 != nil {
				t.Errorf("stats: %+v", st)
			}
		})
	}
}

func TestComputeFuelStatsEdges(t *testing.T) {
	if st := ComputeFuelStats(nil, nil); st.FillUps != 0 || st.ConsumptionL100 != nil || len(st.Logs) != 0 || st.AvgPricePerLiter != 0 {
		t.Errorf("empty: %+v", st)
	}
	// A single full tank, or partial fills only, never yields a consumption.
	if st := ComputeFuelStats([]models.FuelLog{fill(1000, 50, lit(30), true)}, nil); st.ConsumptionL100 != nil || st.MeasurableCount != 0 {
		t.Errorf("single: %+v", st)
	}
	if st := ComputeFuelStats([]models.FuelLog{fill(1000, 50, lit(30), false), fill(1500, 50, lit(30), false)}, nil); st.ConsumptionL100 != nil {
		t.Errorf("partial only: %+v", st)
	}
	// Input order does not matter and the input slice is not modified.
	in := []models.FuelLog{fill(1500, 40, lit(25), true), fill(1000, 60, lit(40), true)}
	st := ComputeFuelStats(in, nil)
	if *in[0].Odometer != 1500 || *st.Logs[0].Odometer != 1000 || st.ConsumptionL100 == nil || *st.ConsumptionL100 != 5 {
		t.Errorf("ordering: %+v", st)
	}
	// Amount-only fill-ups count in the cost but not in the price per litre.
	if st := ComputeFuelStats([]models.FuelLog{fill(1000, 50, nil, true), fill(1500, 40, lit(20), true)}, nil); st.TotalCost != eur(90) || st.AvgPricePerLiter != 2 {
		t.Errorf("mixed: %+v", st)
	}
}

func TestEstimateOdometer(t *testing.T) {
	refs := []OdometerRef{
		{Date: fillDay(0), Odometer: 10000},
		{Date: fillDay(10), Odometer: 10500},
		{Date: fillDay(30), Odometer: 11500},
	}
	tests := []struct {
		name   string
		at     time.Time
		want   float64
		wantOK bool
	}{
		{"midway in the first interval", fillDay(5), 10250, true},
		{"second interval", fillDay(20), 11000, true},
		{"exactly on a reference", fillDay(10), 10500, true},
		{"before the first reference", fillDay(-1), 0, false},
		{"after the last reference", fillDay(31), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := EstimateOdometer(tt.at, refs)
			if ok != tt.wantOK || (ok && got != tt.want) {
				t.Errorf("EstimateOdometer = %v, %v; want %v, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
	if _, ok := EstimateOdometer(fillDay(5), nil); ok {
		t.Error("no reference must not yield an estimate")
	}
	// Two readings on the same day: the fill-up of that day takes the first-then-last value without dividing by zero.
	same := []OdometerRef{{Date: fillDay(3), Odometer: 100}, {Date: fillDay(3), Odometer: 110}}
	if got, ok := EstimateOdometer(fillDay(3), same); !ok || got < 100 || got > 110 {
		t.Errorf("same-day references: %v, %v", got, ok)
	}
}

func TestComputeFuelStatsEstimatedMileage(t *testing.T) {
	refs := []OdometerRef{
		{Date: fillDay(0), Odometer: 10000},
		{Date: fillDay(20), Odometer: 11000}, // 50 km/day
	}
	st := ComputeFuelStats([]models.FuelLog{
		fillNoKm(2, 80, lit(50), true),  // ~10100
		fillNoKm(12, 40, lit(25), true), // ~10600 -> 500 km, 25 L -> 5 L/100 (estimated)
		fillNoKm(30, 40, lit(24), true), // after the last reference: no estimate
	}, refs)

	if st.NoMileageCount != 3 {
		t.Fatalf("fill-ups without mileage = %d, want 3", st.NoMileageCount)
	}
	if got := st.Logs[0].OdometerEstimated; got == nil || *got != 10100 {
		t.Errorf("first estimated mileage = %v, want 10100", got)
	}
	if st.Logs[0].Odometer != nil {
		t.Error("an estimate must never be written into the entered mileage")
	}
	second := st.Logs[1]
	if second.ConsumptionL100 == nil || *second.ConsumptionL100 != 5 || !second.SegmentEstimated {
		t.Errorf("second fill-up: %+v", second)
	}
	if st.Logs[2].OdometerEstimated != nil || st.Logs[2].ConsumptionL100 != nil {
		t.Errorf("no estimate beyond the last reference: %+v", st.Logs[2])
	}
	if st.MeasurableCount != 1 || st.EstimatedCount != 1 || st.UnmeasurableCnt != 1 {
		t.Errorf("segments: measurable=%d estimated=%d unmeasurable=%d", st.MeasurableCount, st.EstimatedCount, st.UnmeasurableCnt)
	}

	// A fill-up entered with its mileage is never given an estimate.
	withKm := ComputeFuelStats([]models.FuelLog{fill(10000, 80, lit(50), true)}, refs)
	if withKm.Logs[0].OdometerEstimated != nil || withKm.NoMileageCount != 0 {
		t.Errorf("entered mileage must be kept as is: %+v", withKm.Logs[0])
	}
}

func TestBuildOdometerRefs(t *testing.T) {
	start := 5000.0
	own := &models.VehicleOwnership{StartDate: fillDay(0), StartOdometer: &start}
	readings := []models.OdometerCheckpoint{{Date: fillDay(10), Odometer: 5400}}
	refs := BuildOdometerRefs(readings, own)
	if len(refs) != 2 || refs[0].Odometer != 5000 || refs[1].Odometer != 5400 {
		t.Errorf("refs = %+v", refs)
	}
	if refs := BuildOdometerRefs(nil, nil); len(refs) != 0 {
		t.Errorf("no ownership and no readings: %+v", refs)
	}
}
