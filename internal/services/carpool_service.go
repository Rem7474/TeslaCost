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
	ElectricityPerKwh float64
	TiresPerKm        float64
	MaintenancePerKm  float64
	InsurancePerKm    float64
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

	// Tires rate (€/km)
	var totalTiresCost float64
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(purchase_price), 0)
		FROM tires
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&totalTiresCost)

	tiresRate := 0.020 // Default fallback ~2.0 cent/km
	if totalDistance > 500 && totalTiresCost > 0 {
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

	// Insurance rate (€/km)
	var totalInsCost float64
	_ = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM maintenance_expenses
		WHERE vehicle_id = $1 AND category = 'INSURANCE';
	`, vehicleID).Scan(&totalInsCost)

	insRate := 0.035 // Default fallback ~3.5 cent/km
	if totalDistance > 500 && totalInsCost > 0 {
		insRate = totalInsCost / totalDistance
	}

	return &UnitRates{
		ElectricityPerKwh: math.Round(elecRate*1000) / 1000,
		TiresPerKm:        math.Round(tiresRate*1000) / 1000,
		MaintenancePerKm:  math.Round(maintRate*1000) / 1000,
		InsurancePerKm:    math.Round(insRate*1000) / 1000,
	}, nil
}

// EstimateCosts calculates suggested cost components for a trip.
func (s *CarpoolService) EstimateCosts(ctx context.Context, vehicleID string, driveID, tripGroupID *string, manualDistanceKm float64) (*models.CarpoolCostEstimate, error) {
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
	}, nil
}
