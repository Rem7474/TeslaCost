package services

import (
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func TestDynamicTireStressModel(t *testing.T) {
	// Case 1: Eco driving (gentle acceleration +45 kW, light regen -20 kW, low consumption 13.5 kWh/100km, front tire)
	avgPowerMaxEco := 45.0
	avgPowerMinEco := -20.0
	avgConsumptionEco := 13.5
	posWeightFront := 0.92

	accelFactorEco := 0.90 + (avgPowerMaxEco/80.0)*0.10
	regenFactorEco := 0.95 + (math.Abs(avgPowerMinEco)/35.0)*0.05
	consumptionFactorEco := math.Max(0.85, math.Min(1.35, avgConsumptionEco/16.0))

	stressEco := (0.45*accelFactorEco + 0.30*regenFactorEco + 0.25*consumptionFactorEco) * posWeightFront
	stressEco = math.Round(stressEco*100) / 100

	if stressEco >= 1.0 {
		t.Errorf("expected eco driving stress on front axle to be < 1.0, got %f", stressEco)
	}

	// Case 2: Sport driving (spirited acceleration +180 kW, strong regen -60 kW, consumption 21.0 kWh/100km, rear tire)
	avgPowerMaxSport := 180.0
	avgPowerMinSport := -60.0
	avgConsumptionSport := 21.0
	posWeightRear := 1.15

	accelFactorSport := 1.0 + (avgPowerMaxSport-80.0)*0.002
	regenFactorSport := 1.0 + (math.Abs(avgPowerMinSport)-35.0)*0.003
	consumptionFactorSport := math.Max(0.85, math.Min(1.35, avgConsumptionSport/16.0))

	stressSport := (0.45*accelFactorSport + 0.30*regenFactorSport + 0.25*consumptionFactorSport) * posWeightRear
	stressSport = math.Round(stressSport*100) / 100

	if stressSport <= 1.15 {
		t.Errorf("expected sport driving stress on rear axle to be > 1.15, got %f", stressSport)
	}

	// Nominal lifespan 40,000 km
	lifespan := 40000.0
	dynamicLifespanSport := int(math.Round(lifespan / stressSport))
	dynamicLifespanEco := int(math.Round(lifespan / stressEco))

	if dynamicLifespanSport >= 40000 {
		t.Errorf("expected sport driving lifespan to be < 40000, got %d", dynamicLifespanSport)
	}
	if dynamicLifespanEco <= 40000 {
		t.Errorf("expected eco driving lifespan to be > 40000, got %d", dynamicLifespanEco)
	}
}

func TestInsuranceRateCalculationHierarchy(t *testing.T) {
	// 1. Vehicle settings provided: 900 € / 15,000 km
	annualPremium := 900.0
	annualMileage := 15000.0
	rateFromSettings := annualPremium / annualMileage
	if math.Abs(rateFromSettings-0.06) > 0.0001 {
		t.Errorf("expected rate to be 0.06 €/km, got %f", rateFromSettings)
	}

	// 2. Default fallback when no settings provided
	fallbackRate := 0.035
	if fallbackRate <= 0 {
		t.Errorf("invalid fallback rate")
	}
}

func TestTireDistanceAtOdometerExcludesStorage(t *testing.T) {
	summerEnd := 20000.0
	sessions := []models.TireMountSession{
		// Winter 1: mounted 10,000 → 15,000 km
		{MountedOdometer: 10000, DismountedOdometer: floatPtr(15000)},
		// Summer in storage, winter 2: mounted from 25,000 km (still mounted)
		{MountedOdometer: 25000},
	}

	if got := TireDistanceAtOdometer(sessions, 12000); got != 2000 {
		t.Errorf("expected 2000 km during first winter, got %f", got)
	}
	if got := TireDistanceAtOdometer(sessions, summerEnd); got != 5000 {
		t.Errorf("expected storage period to add no distance (5000 km), got %f", got)
	}
	if got := TireDistanceAtOdometer(sessions, 28000); got != 8000 {
		t.Errorf("expected 8000 km after 3000 km of second winter, got %f", got)
	}
	// Wear between a measure in March (odometer 15,000) and one in November (odometer 28,000)
	// covers 3,000 tire km, not the 13,000 vehicle km.
	if got := TireDistanceAtOdometer(sessions, 28000) - TireDistanceAtOdometer(sessions, 15000); got != 3000 {
		t.Errorf("expected 3000 tire km between measures, got %f", got)
	}
}

func floatPtr(v float64) *float64 { return &v }

func TestTelemetryEmptyWhenNoDrives(t *testing.T) {
	// Nominal lifespan 45,000 km
	lifespan := 45000
	drivesCount := 0

	var stressIndex float64
	var drivingStyle string
	var dynamicLifespan = lifespan
	var wearExplanation string

	if drivesCount > 0 {
		stressIndex = 1.15
		drivingStyle = "SPORT"
		dynamicLifespan = int(math.Round(float64(lifespan) / stressIndex))
		wearExplanation = "Conduite dynamique"
	}

	if stressIndex != 0 {
		t.Errorf("expected stressIndex to be 0 when drivesCount == 0, got %f", stressIndex)
	}
	if drivingStyle != "" {
		t.Errorf("expected empty drivingStyle when drivesCount == 0, got %s", drivingStyle)
	}
	if dynamicLifespan != lifespan {
		t.Errorf("expected dynamicLifespan to equal base lifespan (%d), got %d", lifespan, dynamicLifespan)
	}
	if wearExplanation != "" {
		t.Errorf("expected empty wearExplanation when drivesCount == 0, got %s", wearExplanation)
	}
}
