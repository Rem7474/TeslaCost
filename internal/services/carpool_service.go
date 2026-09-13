package services

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
)

type CarpoolService struct {
	pool *pgxpool.Pool
	repo *database.Repository
}

func NewCarpoolService(pool *pgxpool.Pool, repo *database.Repository) *CarpoolService {
	return &CarpoolService{
		pool: pool,
		repo: repo,
	}
}

// Rate sources exposed to the UI so that estimates are never mistaken for measured costs.
const (
	RateSourceHistory       = "HISTORY"
	RateSourceMountedTires  = "MOUNTED_TIRES"
	RateSourceDefault       = "DEFAULT"
	EnergySourceMeasured    = "MEASURED"
	EnergySourceConsumption = "CONSUMPTION"
	EnergySourceDefault     = "DEFAULT"
)

// Fallback assumptions used only when the vehicle has no usable history.
const (
	defaultElectricityPerKwh = 0.22
	defaultTiresPerKm        = 0.020
	defaultMaintenancePerKm  = 0.015
	defaultInsurancePerKm    = 0.035
	defaultConsumptionKwh100 = 16.5
	defaultAnnualMileageKm   = 15000.0
)

type UnitRates struct {
	ElectricityPerKwh     float64
	ElectricitySource     string
	TiresPerKm            float64
	TiresSource           string
	MaintenancePerKm      float64
	MaintenanceSource     string
	InsurancePerKm        float64
	InsuranceSource       string // "VEHICLE_SETTINGS", "RECORDED_EXPENSES", "DEFAULT"
	AnnualInsuranceCost   *float64
	AnnualExpectedMileage *float64
}

// DefaultUnitRates returns the fallback assumptions, all flagged as defaults.
func DefaultUnitRates() *UnitRates {
	return &UnitRates{
		ElectricityPerKwh: defaultElectricityPerKwh,
		ElectricitySource: RateSourceDefault,
		TiresPerKm:        defaultTiresPerKm,
		TiresSource:       RateSourceDefault,
		MaintenancePerKm:  defaultMaintenancePerKm,
		MaintenanceSource: RateSourceDefault,
		InsurancePerKm:    defaultInsurancePerKm,
		InsuranceSource:   InsuranceSourceDefault,
	}
}

// DriveEnergyKwh returns the energy of a drive and how it was obtained.
func DriveEnergyKwh(distanceKm float64, energyKwh, consumptionKwh100km *float64) (float64, string) {
	switch {
	case energyKwh != nil && *energyKwh > 0:
		return *energyKwh, EnergySourceMeasured
	case consumptionKwh100km != nil && *consumptionKwh100km > 0:
		return distanceKm * *consumptionKwh100km / 100.0, EnergySourceConsumption
	case distanceKm > 0:
		return distanceKm * defaultConsumptionKwh100 / 100.0, EnergySourceDefault
	}
	return 0, EnergySourceMeasured
}

// GetVehicleUnitRates calculates real cost rates based on vehicle history, with flagged fallbacks.
// Insurance follows the same source priority as the TCO: recorded expenses first, then vehicle settings.
func (s *CarpoolService) GetVehicleUnitRates(ctx context.Context, vehicleID string) (*UnitRates, error) {
	rates := DefaultUnitRates()

	var totalDistance, lastYearDistance float64
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(distance_km), 0),
		       COALESCE(SUM(distance_km) FILTER (WHERE start_time >= NOW() - INTERVAL '365 days'), 0)
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL;
	`, vehicleID).Scan(&totalDistance, &lastYearDistance); err != nil {
		return nil, err
	}

	// Electricity rate (€/kWh), only over charges whose cost is known
	var pricedCost, pricedKwh float64
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE WHEN currency = 'EUR' THEN cost ELSE cost * fx_rate END), 0),
		       COALESCE(SUM(kwh_added), 0)
		FROM charge_logs
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND cost IS NOT NULL AND (currency = 'EUR' OR fx_rate IS NOT NULL);
	`, vehicleID).Scan(&pricedCost, &pricedKwh); err != nil {
		return nil, err
	}
	if pricedKwh > 0 && pricedCost > 0 {
		rates.ElectricityPerKwh = pricedCost / pricedKwh
		rates.ElectricitySource = RateSourceHistory
	}

	// Tires rate (€/km): mounted tires purchase price over their remaining expected life
	var mountedTireRate, totalTiresCost float64
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(purchase_price / GREATEST(estimated_lifespan_km - initial_distance_km, 1))
		                FILTER (WHERE current_position IN ('FL', 'FR', 'RL', 'RR')), 0),
		       COALESCE(SUM(purchase_price), 0)
		FROM tires
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&mountedTireRate, &totalTiresCost); err != nil {
		return nil, err
	}
	if mountedTireRate > 0 {
		rates.TiresPerKm = mountedTireRate
		rates.TiresSource = RateSourceMountedTires
	} else if totalDistance > 500 && totalTiresCost > 0 {
		rates.TiresPerKm = totalTiresCost / totalDistance
		rates.TiresSource = RateSourceHistory
	}

	// Maintenance and insurance from expanded occurrences
	var totalMaintCost, annualInsuranceExpenses float64
	var insuranceEntries int
	if err := s.pool.QueryRow(ctx, `
		WITH `+maintenanceOccurrencesCTE+`
		SELECT COALESCE((SELECT SUM(amount_eur) FROM maintenance_occurrences WHERE category = 'MAINTENANCE'), 0),
		       COALESCE((SELECT SUM(amount_eur) FROM maintenance_occurrences
		                 WHERE category = 'INSURANCE' AND occ_date > NOW() - INTERVAL '365 days'), 0),
		       (SELECT COUNT(*) FROM maintenance_expenses WHERE vehicle_id = $1 AND category = 'INSURANCE');
	`, vehicleID).Scan(&totalMaintCost, &annualInsuranceExpenses, &insuranceEntries); err != nil {
		return nil, err
	}
	if totalDistance > 500 && totalMaintCost > 0 {
		rates.MaintenancePerKm = totalMaintCost / totalDistance
		rates.MaintenanceSource = RateSourceHistory
	}

	var vAnnualIns, vAnnualKm *float64
	if err := s.pool.QueryRow(ctx, `
		SELECT annual_insurance_cost, annual_expected_mileage
		FROM vehicles
		WHERE id = $1;
	`, vehicleID).Scan(&vAnnualIns, &vAnnualKm); err != nil {
		return nil, err
	}

	expectedKm := defaultAnnualMileageKm
	if vAnnualKm != nil && *vAnnualKm > 0 {
		expectedKm = *vAnnualKm
	} else if lastYearDistance > 1000 {
		expectedKm = lastYearDistance
	}

	switch {
	case insuranceEntries > 0 && annualInsuranceExpenses > 0:
		rates.InsurancePerKm = annualInsuranceExpenses / expectedKm
		rates.InsuranceSource = InsuranceSourceRecordedExpenses
		rates.AnnualInsuranceCost = &annualInsuranceExpenses
		rates.AnnualExpectedMileage = &expectedKm
	case vAnnualIns != nil && *vAnnualIns > 0:
		rates.InsurancePerKm = *vAnnualIns / expectedKm
		rates.InsuranceSource = InsuranceSourceVehicleSettings
		rates.AnnualInsuranceCost = vAnnualIns
		rates.AnnualExpectedMileage = &expectedKm
	}

	rates.ElectricityPerKwh = round3(rates.ElectricityPerKwh)
	rates.TiresPerKm = round3(rates.TiresPerKm)
	rates.MaintenancePerKm = round3(rates.MaintenancePerKm)
	rates.InsurancePerKm = round3(rates.InsurancePerKm)
	return rates, nil
}

// EstimateCosts calculates suggested cost components for a trip.
func (s *CarpoolService) EstimateCosts(ctx context.Context, vehicleID string, driveID, tripGroupID *string, driveIDs []string, manualDistanceKm float64) (*models.CarpoolCostEstimate, error) {
	rates, err := s.GetVehicleUnitRates(ctx, vehicleID)
	if err != nil {
		return nil, err
	}

	var drives []models.Drive
	switch {
	case driveID != nil && *driveID != "":
		d, err := s.repo.GetDriveByID(ctx, *driveID, vehicleID)
		if err != nil {
			return nil, err
		}
		drives = []models.Drive{*d}
	case tripGroupID != nil && *tripGroupID != "":
		if drives, err = s.repo.GetTripGroupDrives(ctx, vehicleID, *tripGroupID); err != nil {
			return nil, err
		}
		if len(drives) == 0 {
			return nil, database.ErrNotFound
		}
	case len(driveIDs) > 0:
		for _, did := range driveIDs {
			d, err := s.repo.GetDriveByID(ctx, did, vehicleID)
			if err != nil {
				return nil, err
			}
			drives = append(drives, *d)
		}
	}

	var distance, kwhConsumed, tolls float64
	energySource := EnergySourceMeasured
	if len(drives) > 0 {
		ids := make([]string, len(drives))
		for i, d := range drives {
			ids[i] = d.ID
			distance += d.DistanceKm
			kwh, src := DriveEnergyKwh(d.DistanceKm, d.EnergyConsumedKwh, d.ConsumptionKwh100km)
			kwhConsumed += kwh
			if src != EnergySourceMeasured {
				energySource = src
			}
		}
		if tolls, err = s.repo.GetTotalTollExpensesForDrives(ctx, vehicleID, ids); err != nil {
			return nil, err
		}
	} else {
		distance = manualDistanceKm
		kwhConsumed, energySource = DriveEnergyKwh(distance, nil, nil)
	}

	elecCost := round2(kwhConsumed * rates.ElectricityPerKwh)
	tiresCost := round2(distance * rates.TiresPerKm)
	maintCost := round2(distance * rates.MaintenancePerKm)
	insCost := round2(distance * rates.InsurancePerKm)
	totalCost := elecCost + tolls + tiresCost + maintCost + insCost

	return &models.CarpoolCostEstimate{
		DistanceKm:            round1(distance),
		ElectricityCost:       elecCost,
		TollsCost:             round2(tolls),
		TiresCost:             tiresCost,
		MaintenanceCost:       maintCost,
		InsuranceCost:         insCost,
		OtherCost:             0.0,
		TotalCost:             round2(totalCost),
		ElectricityRatePerKwh: rates.ElectricityPerKwh,
		TiresRatePerKm:        rates.TiresPerKm,
		MaintenanceRatePerKm:  rates.MaintenancePerKm,
		InsuranceRatePerKm:    rates.InsurancePerKm,
		InsuranceSource:       rates.InsuranceSource,
		AnnualInsuranceCost:   rates.AnnualInsuranceCost,
		AnnualExpectedMileage: rates.AnnualExpectedMileage,
		EnergySource:          energySource,
		ElectricityRateSource: rates.ElectricitySource,
		TiresRateSource:       rates.TiresSource,
		MaintenanceRateSource: rates.MaintenanceSource,
	}, nil
}
