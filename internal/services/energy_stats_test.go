package services

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/money"
)

func ptrF(v float64) *float64 { return &v }

func ptrCents(v float64) *money.Cents {
	c := money.FromFloat(v)
	return &c
}

func session(month string, hours float64, kwhAdded float64, kwhUsed *float64, cost *money.Cents) energyCharge {
	start := time.Date(2026, 1, 10, 18, 0, 0, 0, time.UTC)
	end := start.Add(time.Duration(hours * float64(time.Hour)))
	return energyCharge{Month: month, Start: start, End: &end, KwhAdded: kwhAdded, KwhUsed: kwhUsed, Cost: cost}
}

func eq(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: got nil, want %v", name, want)
		return
	}
	if *got != want {
		t.Errorf("%s: got %v, want %v", name, *got, want)
	}
}

func isNil(t *testing.T, name string, got *float64) {
	t.Helper()
	if got != nil {
		t.Errorf("%s: got %v, want nil", name, *got)
	}
}

func TestClassifyCharge(t *testing.T) {
	start := time.Date(2026, 1, 10, 18, 0, 0, 0, time.UTC)
	after := func(h float64) *time.Time {
		e := start.Add(time.Duration(h * float64(time.Hour)))
		return &e
	}
	zero := start

	tests := []struct {
		name  string
		kwh   float64
		end   *time.Time
		class string
		ok    bool
	}{
		{"domestic socket 2.3 kW", 11.5, after(5), ChargeClassSlow, true},
		{"wallbox 7.4 kW", 22.2, after(3), ChargeClassAC, true},
		{"supercharger 120 kW", 60, after(0.5), ChargeClassDC, true},
		{"exactly the slow limit is AC", 3.5, after(1), ChargeClassAC, true},
		{"exactly the AC limit is DC", 25, after(1), ChargeClassDC, true},
		{"manual entry without end", 30, nil, ChargeClassUnknown, false},
		{"zero duration", 30, &zero, ChargeClassUnknown, false},
		{"no energy", 0, after(1), ChargeClassUnknown, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			class, ok := classifyCharge(tt.kwh, start, tt.end)
			if class != tt.class || ok != tt.ok {
				t.Errorf("got (%s, %v), want (%s, %v)", class, ok, tt.class, tt.ok)
			}
		})
	}
}

func TestEfficiencyRatioRejectsImplausibleReadings(t *testing.T) {
	if r, ok := efficiencyRatio(30, ptrF(33)); !ok || r < 0.909 || r > 0.910 {
		t.Errorf("typical AC session: got (%v, %v)", r, ok)
	}
	for name, used := range map[string]*float64{
		"grid energy missing":                              nil,
		"grid energy zero":                                 ptrF(0),
		"stored more than drawn":                           ptrF(25),
		"grid energy equal to energy added (not measured)": ptrF(30),
		"far more drawn than stored":                       ptrF(60),
		"just below the plausible minimum":                 ptrF(30 / 0.59),
	} {
		if _, ok := efficiencyRatio(30, used); ok {
			t.Errorf("%s: reading accepted", name)
		}
	}
}

func TestPreviousMonthsAcrossYearBoundary(t *testing.T) {
	got := previousMonths("2026-02", 3)
	want := []string{"2025-12", "2026-01", "2026-02"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestComputeEnergyStats(t *testing.T) {
	drives := []energyDriveMonth{
		{Month: "2026-01", DistanceKm: 1000, MeasuredKm: 800, Kwh: 120}, // 200 km without energy reading
		{Month: "2026-02", DistanceKm: 500, MeasuredKm: 500, Kwh: 80},
		{Month: "2026-03", DistanceKm: 40, MeasuredKm: 40, Kwh: 6}, // too short for a cost per 100 km
	}
	charges := []energyCharge{
		session("2025-12", 4, 30, nil, ptrCents(9)),      // before TeslaMate: cost without distance
		session("2026-01", 4, 30, ptrF(33), ptrCents(6)), // 7.5 kW: AC
		session("2026-01", 0.5, 40, nil, ptrCents(20)),   // 80 kW: DC, grid energy not reported
		session("2026-02", 5, 10, ptrF(12), ptrCents(2)), // 2 kW: socket
		session("2026-02", 3, 20, ptrF(22), nil),         // AC, no tariff known
	}

	got := computeEnergyStats(drives, charges)

	if len(got.Months) != 4 {
		t.Fatalf("months: got %d, want 4 (Dec to Mar)", len(got.Months))
	}
	dec, jan, feb, mar := got.Months[0], got.Months[1], got.Months[2], got.Months[3]

	// January
	eq(t, "jan consumption", jan.ConsumptionKwh100km, 15.0) // only the 800 km with a reading count
	eq(t, "jan cost/100km", jan.CostPer100km, 2.6)
	eq(t, "jan price/kWh", jan.PricePerKwh, 0.371)
	eq(t, "jan charge efficiency", jan.ChargeEfficiency, 0.909) // the DC session has no grid reading
	eq(t, "jan trailing", jan.CostPer100kmTrailing, 2.6)        // December has no distance: left out
	if jan.EnergyCost != money.FromFloat(26) {
		t.Errorf("jan energy cost: got %v", jan.EnergyCost)
	}

	// February: only the priced session counts towards cost
	eq(t, "feb consumption", feb.ConsumptionKwh100km, 16.0)
	eq(t, "feb cost/100km", feb.CostPer100km, 0.4)
	eq(t, "feb trailing", feb.CostPer100kmTrailing, 1.87) // 28 EUR over 1500 km
	eq(t, "feb price/kWh", feb.PricePerKwh, 0.2)          // 2 EUR over the 10 priced kWh only

	// Months without enough distance give no cost per 100 km
	isNil(t, "dec cost/100km", dec.CostPer100km)
	eq(t, "dec price/kWh", dec.PricePerKwh, 0.3)
	isNil(t, "mar cost/100km", mar.CostPer100km)
	eq(t, "mar trailing", mar.CostPer100kmTrailing, 1.82) // the window still spans Jan to Mar: 28 EUR over 1540 km

	// Summary
	eq(t, "summary consumption", got.Summary.ConsumptionKwh100km, 15.4)
	eq(t, "summary cost/100km", got.Summary.CostPer100km, 1.82) // December energy excluded: no matching distance
	eq(t, "summary price/kWh", got.Summary.PricePerKwh, 0.336)
	eq(t, "summary efficiency", got.Summary.ChargeEfficiency, 0.896)
	if got.Summary.SessionsWithoutCost != 1 {
		t.Errorf("sessions without cost: got %d, want 1", got.Summary.SessionsWithoutCost)
	}

	// Classes, in a fixed order
	if len(got.ChargeClasses) != 3 {
		t.Fatalf("classes: got %+v", got.ChargeClasses)
	}
	slow, ac, dc := got.ChargeClasses[0], got.ChargeClasses[1], got.ChargeClasses[2]
	if slow.Class != ChargeClassSlow || ac.Class != ChargeClassAC || dc.Class != ChargeClassDC {
		t.Fatalf("class order: %s %s %s", slow.Class, ac.Class, dc.Class)
	}
	if ac.Sessions != 3 || ac.KwhAdded != 80 {
		t.Errorf("AC: got %d sessions / %v kWh", ac.Sessions, ac.KwhAdded)
	}
	eq(t, "AC price/kWh", ac.PricePerKwh, 0.25) // 15 EUR over the 60 priced kWh
	eq(t, "DC price/kWh", dc.PricePerKwh, 0.5)
	isNil(t, "DC efficiency", dc.ChargeEfficiency)
	eq(t, "socket efficiency", slow.ChargeEfficiency, 0.833)
}

func TestComputeEnergyStatsWithoutData(t *testing.T) {
	got := computeEnergyStats(nil, nil)
	if got.Months == nil || got.ChargeClasses == nil {
		t.Fatal("empty statistics must serialise as empty lists, not null")
	}
	if len(got.Months) != 0 || len(got.ChargeClasses) != 0 {
		t.Errorf("got %+v", got)
	}
	isNil(t, "consumption", got.Summary.ConsumptionKwh100km)
	isNil(t, "cost/100km", got.Summary.CostPer100km)
}

func TestComputeEnergyStatsUnknownClassForManualCharges(t *testing.T) {
	start := time.Date(2026, 1, 10, 18, 0, 0, 0, time.UTC)
	got := computeEnergyStats(nil, []energyCharge{{Month: "2026-01", Start: start, KwhAdded: 25, Cost: ptrCents(7.5)}})
	if len(got.ChargeClasses) != 1 || got.ChargeClasses[0].Class != ChargeClassUnknown {
		t.Fatalf("got %+v", got.ChargeClasses)
	}
	eq(t, "unknown price/kWh", got.ChargeClasses[0].PricePerKwh, 0.3)
}
