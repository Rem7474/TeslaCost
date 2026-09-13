package services

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
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
	RateSourceIncluded      = "INCLUDED_IN_LEASE"
	EnergySourceMeasured    = "MEASURED"
	EnergySourceConsumption = "CONSUMPTION"
	EnergySourceDefault     = "DEFAULT"
)

// Fallback assumptions used only when the vehicle has no usable history.
const (
	defaultElectricityPerKwh = 0.22
	defaultTiresPerKm        = 0.020
	defaultMaintenancePerKm  = 0.015
	defaultConsumptionKwh100 = 16.5
	// Minimum distance over the insurance window for a meaningful per-km share.
	minInsuranceWindowKm = 500.0
)

type UnitRates struct {
	ElectricityPerKwh float64
	ElectricitySource string
	TiresPerKm        float64
	TiresSource       string
	MaintenancePerKm  float64
	MaintenanceSource string
	// InsurancePerKm shares the insurance premiums paid over the last 12 months (or since the first premium)
	// across the kilometers actually driven over the same window.
	InsurancePerKm      float64
	InsuranceSource     string // RECORDED_EXPENSES | INCLUDED_IN_LEASE | INSUFFICIENT_DISTANCE | NONE
	InsuranceWindowCost *money.Cents
	InsuranceWindowKm   *float64
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
		InsuranceSource:   InsuranceSourceNone,
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
	var pricedCost money.Cents
	var pricedKwh float64
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE WHEN currency = 'EUR' THEN cost ELSE cost * fx_rate END), 0),
		       COALESCE(SUM(kwh_added), 0)
		FROM charge_logs
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND cost IS NOT NULL AND (currency = 'EUR' OR fx_rate IS NOT NULL);
	`, vehicleID).Scan(&pricedCost, &pricedKwh); err != nil {
		return nil, err
	}
	if pricedKwh > 0 && pricedCost > 0 {
		rates.ElectricityPerKwh = pricedCost.Float() / pricedKwh
		rates.ElectricitySource = RateSourceHistory
	}

	// Tires rate (€/km): mounted tires purchase price over their remaining expected life
	var mountedTireRate float64
	var totalTiresCost money.Cents
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
		rates.TiresPerKm = totalTiresCost.Float() / totalDistance
		rates.TiresSource = RateSourceHistory
	}

	// Maintenance and repairs from the cost ledger
	var totalMaintCost money.Cents
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount_eur), 0) FROM cost_ledger
		WHERE vehicle_id = $1 AND category IN ('MAINTENANCE', 'REPAIR');
	`, vehicleID).Scan(&totalMaintCost); err != nil {
		return nil, err
	}
	if totalDistance > 500 && totalMaintCost > 0 {
		rates.MaintenancePerKm = totalMaintCost.Float() / totalDistance
		rates.MaintenanceSource = RateSourceHistory
	}

	// Insurance: premiums paid over the window divided by the kilometers driven over the same window
	var insuranceCost money.Cents
	var windowKm float64
	var windowStart *time.Time
	if err := s.pool.QueryRow(ctx, `
		WITH premiums AS (
			SELECT entry_date, amount_eur FROM cost_ledger
			WHERE vehicle_id = $1 AND category = 'INSURANCE' AND entry_date <= NOW()
		),
		window_start AS (
			SELECT GREATEST(NOW() - INTERVAL '365 days', MIN(entry_date)) AS start FROM premiums
		)
		SELECT (SELECT start FROM window_start),
		       COALESCE((SELECT SUM(amount_eur) FROM premiums WHERE entry_date >= (SELECT start FROM window_start)), 0),
		       COALESCE((SELECT SUM(distance_km) FROM drives
		                 WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL
		                   AND start_time >= (SELECT start FROM window_start)), 0);
	`, vehicleID).Scan(&windowStart, &insuranceCost, &windowKm); err != nil {
		return nil, err
	}

	// Services included in a running lease contract
	ownership, err := s.repo.GetVehicleOwnership(ctx, vehicleID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, err
	}
	now := time.Now()
	if ownership != nil && ownership.InLeasePhase(now) {
		if ownership.LeaseIncludesMaintenance {
			rates.MaintenancePerKm, rates.MaintenanceSource = 0, RateSourceIncluded
		}
		if ownership.LeaseIncludesTires {
			rates.TiresPerKm, rates.TiresSource = 0, RateSourceIncluded
		}
	}

	switch {
	case windowStart != nil && insuranceCost > 0 && windowKm >= minInsuranceWindowKm:
		rates.InsurancePerKm = insuranceCost.Float() / windowKm
		rates.InsuranceSource = InsuranceSourceRecordedExpenses
		rates.InsuranceWindowCost = &insuranceCost
		rates.InsuranceWindowKm = &windowKm
	case windowStart != nil && insuranceCost > 0:
		rates.InsuranceSource = InsuranceSourceInsufficientKm
		rates.InsuranceWindowCost = &insuranceCost
		rates.InsuranceWindowKm = &windowKm
	case ownership != nil && ownership.InLeasePhase(now) && ownership.LeaseIncludesInsurance:
		rates.InsuranceSource = InsuranceSourceIncluded
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

	var distance, kwhConsumed float64
	var tolls money.Cents
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

	elecCost := money.FromFloat(kwhConsumed * rates.ElectricityPerKwh)
	tiresCost := money.FromFloat(distance * rates.TiresPerKm)
	maintCost := money.FromFloat(distance * rates.MaintenancePerKm)
	insCost := money.FromFloat(distance * rates.InsurancePerKm)
	totalCost := elecCost + tolls + tiresCost + maintCost + insCost

	return &models.CarpoolCostEstimate{
		DistanceKm:            round1(distance),
		ElectricityCost:       elecCost,
		TollsCost:             tolls,
		TiresCost:             tiresCost,
		MaintenanceCost:       maintCost,
		InsuranceCost:         insCost,
		OtherCost:             0,
		TotalCost:             totalCost,
		ElectricityRatePerKwh: rates.ElectricityPerKwh,
		TiresRatePerKm:        rates.TiresPerKm,
		MaintenanceRatePerKm:  rates.MaintenancePerKm,
		InsuranceRatePerKm:    rates.InsurancePerKm,
		InsuranceSource:       rates.InsuranceSource,
		InsuranceWindowCost:   rates.InsuranceWindowCost,
		InsuranceWindowKm:     rates.InsuranceWindowKm,
		EnergySource:          energySource,
		ElectricityRateSource: rates.ElectricitySource,
		TiresRateSource:       rates.TiresSource,
		MaintenanceRateSource: rates.MaintenanceSource,
	}, nil
}
