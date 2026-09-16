package services

import (
	"testing"

	"github.com/teslacost/teslacost/internal/money"
)

func TestCarpoolUnitRatesDefaults(t *testing.T) {
	rates := &UnitRates{
		ElectricityPerKwh: 0.22,
		TiresPerKm:        0.020,
		MaintenancePerKm:  0.015,
		InsurancePerKm:    0.035,
	}

	distance := 450.0                // Paris - Lyon
	kwh := (distance * 16.5) / 100.0 // 74.25 kWh
	elecCost := kwh * rates.ElectricityPerKwh
	tiresCost := distance * rates.TiresPerKm
	maintCost := distance * rates.MaintenancePerKm
	insCost := distance * rates.InsurancePerKm
	tolls := 35.0

	totalCost := elecCost + tolls + tiresCost + maintCost + insCost

	if totalCost <= 0 {
		t.Errorf("expected positive total cost, got %f", totalCost)
	}

	// 3 passengers paying total 65 EUR
	revenue := 65.0
	netCost := totalCost - revenue
	coveragePct := (revenue / totalCost) * 100

	if coveragePct <= 0 || coveragePct > 100 {
		t.Errorf("expected coverage percentage between 0 and 100, got %f", coveragePct)
	}

	if netCost >= totalCost {
		t.Errorf("expected net cost to be reduced by passengers revenue")
	}
}

func TestDailyInsuranceAllocationFormula(t *testing.T) {
	annualInsuranceEUR := 365.25
	dailyInsuranceEUR := annualInsuranceEUR / 365.25 // 1.00 EUR/day

	// Day 1: 40 km leg 1, 60 km leg 2, 100 km other driving -> total 200 km
	totalDay1Km := 200.0
	leg1Km := 40.0
	leg2Km := 60.0

	leg1Cost := money.FromFloat(dailyInsuranceEUR * (leg1Km / totalDay1Km))
	leg2Cost := money.FromFloat(dailyInsuranceEUR * (leg2Km / totalDay1Km))

	if leg1Cost != 20 {
		t.Fatalf("expected 20 cents, got %d", leg1Cost)
	}
	if leg2Cost != 30 {
		t.Fatalf("expected 30 cents, got %d", leg2Cost)
	}
	totalTripCost := leg1Cost + leg2Cost
	if totalTripCost != 50 {
		t.Fatalf("expected 50 cents, got %d", totalTripCost)
	}

	// Day 2: 50 km leg, 50 km total (only driving of the day)
	totalDay2Km := 50.0
	leg3Km := 50.0
	leg3Cost := money.FromFloat(dailyInsuranceEUR * (leg3Km / totalDay2Km))
	if leg3Cost != 100 {
		t.Fatalf("expected 100 cents (100%% of daily insurance), got %d", leg3Cost)
	}
}
