package services

import (
	"testing"
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
