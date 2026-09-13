package services

import (
	"context"
	"math"

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

type UnitRates struct {
	ElectricityPerKwh     float64
	TiresPerKm            float64
	MaintenancePerKm      float64
	InsurancePerKm        float64
	InsuranceSource       string // "VEHICLE_SETTINGS", "RECORDED_EXPENSES", "DEFAULT"
	AnnualInsuranceCost   *float64
	AnnualExpectedMileage *float64
}

// GetVehicleUnitRates calculates real cost rates based on vehicle history, with sensible fallbacks.
func (s *CarpoolService) GetVehicleUnitRates(ctx context.Context, vehicleID string) (*UnitRates, error) {
	var totalDistance float64
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(distance_km), 0)
		FROM drives
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&totalDistance)

	// Electricity rate (€/kWh)
	var totalEnergyCost, totalKwhAdded float64
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost), 0), COALESCE(SUM(kwh_added), 0)
		FROM charge_logs
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&totalEnergyCost, &totalKwhAdded)

	elecRate := 0.22 // Default fallback ~0.22 €/kWh in Europe
	if totalKwhAdded > 0 && totalEnergyCost > 0 {
		elecRate = totalEnergyCost / totalKwhAdded
	}

	// Tires rate (€/km): based on mounted tires purchase_price / estimated_lifespan_km
	var totalMountedTireRate float64
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(purchase_price / NULLIF(estimated_lifespan_km, 0)), 0)
		FROM tires
		WHERE vehicle_id = $1 AND current_position IN ('FL', 'FR', 'RL', 'RR');
	`, vehicleID).Scan(&totalMountedTireRate)

	var totalTiresCost float64
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(purchase_price), 0)
		FROM tires
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&totalTiresCost)

	tiresRate := 0.020 // Default fallback ~2.0 cent/km for 4 wheels
	if totalMountedTireRate > 0 {
		tiresRate = totalMountedTireRate
	} else if totalDistance > 500 && totalTiresCost > 0 {
		tiresRate = totalTiresCost / totalDistance
	}

	// Maintenance rate (€/km)
	var totalMaintCost float64
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM maintenance_expenses
		WHERE vehicle_id = $1 AND category = 'MAINTENANCE';
	`, vehicleID).Scan(&totalMaintCost)

	maintRate := 0.015 // Default fallback ~1.5 cent/km
	if totalDistance > 500 && totalMaintCost > 0 {
		maintRate = totalMaintCost / totalDistance
	}

	// Real Insurance calculation (€/km):
	// Priority 1: Direct vehicle insurance settings (annual insurance premium / annual expected mileage)
	var vAnnualIns *float64
	var vAnnualKm *float64
	_ = s.pool.QueryRow(ctx, `
		SELECT annual_insurance_cost, annual_expected_mileage
		FROM vehicles
		WHERE id = $1;
	`, vehicleID).Scan(&vAnnualIns, &vAnnualKm)

	insRate := 0.035
	insSource := "DEFAULT"
	var finalAnnualIns *float64
	var finalAnnualKm *float64

	if vAnnualIns != nil && *vAnnualIns > 0 {
		expectedKm := 15000.0
		if vAnnualKm != nil && *vAnnualKm > 0 {
			expectedKm = *vAnnualKm
		}
		insRate = *vAnnualIns / expectedKm
		insSource = "VEHICLE_SETTINGS"
		finalAnnualIns = vAnnualIns
		finalAnnualKm = &expectedKm
	} else {
		// Priority 2: Check recorded INSURANCE expenses and annualize them
		rows, err := s.pool.Query(ctx, `
			SELECT amount, is_recurring, recurrence_interval_months
			FROM maintenance_expenses
			WHERE vehicle_id = $1 AND category = 'INSURANCE'
			ORDER BY date DESC;
		`, vehicleID)
		if err == nil {
			defer rows.Close()
			var annualizedSum float64
			hasEntries := false
			for rows.Next() {
				var amt float64
				var isRec bool
				var recMonths *int
				if err := rows.Scan(&amt, &isRec, &recMonths); err == nil {
					hasEntries = true
					if isRec {
						interval := 1
						if recMonths != nil && *recMonths > 0 {
							interval = *recMonths
						}
						annualizedSum += amt * (12.0 / float64(interval))
					} else {
						annualizedSum += amt
					}
				}
			}
			if hasEntries && annualizedSum > 0 {
				expectedKm := 15000.0
				if vAnnualKm != nil && *vAnnualKm > 0 {
					expectedKm = *vAnnualKm
				} else if totalDistance > 5000 {
					expectedKm = totalDistance
				}
				insRate = annualizedSum / expectedKm
				insSource = "RECORDED_EXPENSES"
				finalAnnualIns = &annualizedSum
				finalAnnualKm = &expectedKm
			}
		}
	}

	return &UnitRates{
		ElectricityPerKwh:     math.Round(elecRate*1000) / 1000,
		TiresPerKm:            math.Round(tiresRate*1000) / 1000,
		MaintenancePerKm:      math.Round(maintRate*1000) / 1000,
		InsurancePerKm:        math.Round(insRate*1000) / 1000,
		InsuranceSource:       insSource,
		AnnualInsuranceCost:   finalAnnualIns,
		AnnualExpectedMileage: finalAnnualKm,
	}, nil
}

// EstimateCosts calculates suggested cost components for a trip.
func (s *CarpoolService) EstimateCosts(ctx context.Context, vehicleID string, driveID, tripGroupID *string, driveIDs []string, manualDistanceKm float64) (*models.CarpoolCostEstimate, error) {
	rates, err := s.GetVehicleUnitRates(ctx, vehicleID)
	if err != nil {
		return nil, err
	}

	var distance float64
	var kwhConsumed float64
	var tolls float64

	if driveID != nil && *driveID != "" {
		drive, err := s.repo.GetDriveByID(ctx, *driveID, vehicleID)
		if err == nil && drive != nil {
			distance = drive.DistanceKm
			if drive.EnergyConsumedKwh != nil {
				kwhConsumed = *drive.EnergyConsumedKwh
			}
		}
		tolls, _ = s.repo.GetTollExpensesForDriveOrGroup(ctx, vehicleID, driveID, nil)
	} else if tripGroupID != nil && *tripGroupID != "" {
		drives, err := s.repo.GetTripGroupDrives(ctx, *tripGroupID)
		if err == nil {
			for _, d := range drives {
				distance += d.DistanceKm
				if d.EnergyConsumedKwh != nil {
					kwhConsumed += *d.EnergyConsumedKwh
				}
			}
		}
		tolls, _ = s.repo.GetTollExpensesForDriveOrGroup(ctx, vehicleID, nil, tripGroupID)
	} else if len(driveIDs) > 0 {
		for _, did := range driveIDs {
			drive, err := s.repo.GetDriveByID(ctx, did, vehicleID)
			if err == nil && drive != nil {
				distance += drive.DistanceKm
				if drive.EnergyConsumedKwh != nil {
					kwhConsumed += *drive.EnergyConsumedKwh
				} else if drive.ConsumptionKwh100km != nil {
					kwhConsumed += (drive.DistanceKm * *drive.ConsumptionKwh100km) / 100.0
				}
			}
		}
		tolls, _ = s.repo.GetTotalTollExpensesForDrives(ctx, vehicleID, driveIDs)
	} else {
		distance = manualDistanceKm
	}

	// Fallback consumption if not recorded by TeslaMate (~16.5 kWh / 100 km for Tesla Model 3/Y)
	if kwhConsumed <= 0 && distance > 0 {
		kwhConsumed = (distance * 16.5) / 100.0
	}

	elecCost := math.Round(kwhConsumed*rates.ElectricityPerKwh*100) / 100
	tiresCost := math.Round(distance*rates.TiresPerKm*100) / 100
	maintCost := math.Round(distance*rates.MaintenancePerKm*100) / 100
	insCost := math.Round(distance*rates.InsurancePerKm*100) / 100
	totalCost := elecCost + tolls + tiresCost + maintCost + insCost

	return &models.CarpoolCostEstimate{
		DistanceKm:            math.Round(distance*10) / 10,
		ElectricityCost:       elecCost,
		TollsCost:             math.Round(tolls*100) / 100,
		TiresCost:             tiresCost,
		MaintenanceCost:       maintCost,
		InsuranceCost:         insCost,
		OtherCost:             0.0,
		TotalCost:             math.Round(totalCost*100) / 100,
		ElectricityRatePerKwh: rates.ElectricityPerKwh,
		TiresRatePerKm:        rates.TiresPerKm,
		MaintenanceRatePerKm:  rates.MaintenancePerKm,
		InsuranceRatePerKm:    rates.InsurancePerKm,
		InsuranceSource:       rates.InsuranceSource,
		AnnualInsuranceCost:   rates.AnnualInsuranceCost,
		AnnualExpectedMileage: rates.AnnualExpectedMileage,
	}, nil
}
