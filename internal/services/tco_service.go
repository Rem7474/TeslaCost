package services

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MonthlyCost represents monthly expenditure and mileage breakdown.
type MonthlyCost struct {
	Month       string  `json:"month"` // YYYY-MM
	DistanceKm  float64 `json:"distance_km"`
	Energy      float64 `json:"energy"`
	Tolls       float64 `json:"tolls"`
	Maintenance float64 `json:"maintenance"`
	Tires       float64 `json:"tires"`
	Total       float64 `json:"total"`
	CostPerKm   float64 `json:"cost_per_km"`
}

// TagCostBreakdown represents costs split by tag (e.g. Pro vs Perso).
type TagCostBreakdown struct {
	Tag         string  `json:"tag"`
	DistanceKm  float64 `json:"distance_km"`
	EnergyKwh   float64 `json:"energy_kwh"`
	TollsAmount float64 `json:"tolls_amount"`
	Percentage  float64 `json:"percentage"`
}

// TCOSummary represents the global TCO calculation.
type TCOSummary struct {
	TotalDistanceKm float64 `json:"total_distance_km"`
	TotalCost       float64 `json:"total_cost"`
	TotalCostPerKm  float64 `json:"total_cost_per_km"`

	// Category breakdowns
	EnergyCost        float64 `json:"energy_cost"`
	EnergyCostPerKm   float64 `json:"energy_cost_per_km"`
	TotalKwhAdded     float64 `json:"total_kwh_added"`
	AvgCostPerKwh     float64 `json:"avg_cost_per_kwh"`

	TollsCost         float64 `json:"tolls_cost"`
	TollsCostPerKm    float64 `json:"tolls_cost_per_km"`

	TiresCost         float64 `json:"tires_cost"`
	TiresCostPerKm    float64 `json:"tires_cost_per_km"`

	MaintenanceCost   float64 `json:"maintenance_cost"`
	MaintenanceCostPerKm float64 `json:"maintenance_cost_per_km"`

	// Tag analysis (Pro / Perso)
	TagBreakdown []TagCostBreakdown `json:"tag_breakdown"`

	// Timeline
	MonthlyCosts []MonthlyCost `json:"monthly_costs"`
}

// TCOService computes TCO metrics using optimized SQL queries.
type TCOService struct {
	pool *pgxpool.Pool
}

// NewTCOService creates a new TCOService.
func NewTCOService(pool *pgxpool.Pool) *TCOService {
	return &TCOService{pool: pool}
}

// ComputeVehicleTCO calculates complete TCO for a given vehicle.
func (s *TCOService) ComputeVehicleTCO(ctx context.Context, vehicleID string) (*TCOSummary, error) {
	// 1. Total distance from drives or vehicle
	var totalDistance float64
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(distance_km), 0)
		FROM drives
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&totalDistance)
	if err != nil {
		return nil, err
	}

	// 2. Charging costs
	var totalEnergyCost, totalKwhAdded float64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(cost), 0), COALESCE(SUM(kwh_added), 0)
		FROM charge_logs
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&totalEnergyCost, &totalKwhAdded)
	if err != nil {
		return nil, err
	}

	// 3. Tolls and parkings costs
	var totalTollsCost float64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM drive_expenses
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&totalTollsCost)
	if err != nil {
		return nil, err
	}

	// 4. Tires purchase cost
	var totalTiresCost float64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(purchase_price), 0)
		FROM tires
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&totalTiresCost)
	if err != nil {
		return nil, err
	}

	// 5. Maintenance and recurring costs
	var totalMaintenanceCost float64
	err = s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(amount), 0)
		FROM maintenance_expenses
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&totalMaintenanceCost)
	if err != nil {
		return nil, err
	}

	// Total TCO
	totalCost := totalEnergyCost + totalTollsCost + totalTiresCost + totalMaintenanceCost

	costPerKm := 0.0
	energyPerKm := 0.0
	tollsPerKm := 0.0
	tiresPerKm := 0.0
	maintPerKm := 0.0

	if totalDistance > 0 {
		costPerKm = totalCost / totalDistance
		energyPerKm = totalEnergyCost / totalDistance
		tollsPerKm = totalTollsCost / totalDistance
		tiresPerKm = totalTiresCost / totalDistance
		maintPerKm = totalMaintenanceCost / totalDistance
	}

	avgCostKwh := 0.0
	if totalKwhAdded > 0 {
		avgCostKwh = totalEnergyCost / totalKwhAdded
	}

	// 6. Tag breakdown (Pro / Perso / Other)
	tagRows, err := s.pool.Query(ctx, `
		SELECT COALESCE(tag, 'Non tagué') AS tag_name,
		       COALESCE(SUM(distance_km), 0) AS total_km,
		       COALESCE(SUM(energy_consumed_kwh), 0) AS total_kwh
		FROM (
			SELECT unnest(CASE WHEN tags = '{}' OR tags IS NULL THEN ARRAY['Non tagué'] ELSE tags END) AS tag,
			       distance_km, energy_consumed_kwh
			FROM drives
			WHERE vehicle_id = $1
		) sub
		GROUP BY tag_name
		ORDER BY total_km DESC;
	`, vehicleID)
	var tagBreakdown []TagCostBreakdown
	if err == nil {
		defer tagRows.Close()
		for tagRows.Next() {
			var tb TagCostBreakdown
			if err := tagRows.Scan(&tb.Tag, &tb.DistanceKm, &tb.EnergyKwh); err == nil {
				if totalDistance > 0 {
					tb.Percentage = math.Round((tb.DistanceKm/totalDistance)*1000) / 10
				}
				tb.DistanceKm = math.Round(tb.DistanceKm*10) / 10
				tb.EnergyKwh = math.Round(tb.EnergyKwh*10) / 10
				tagBreakdown = append(tagBreakdown, tb)
			}
		}
	}

	// 7. Monthly time series (last 12 months)
	monthlyMap := make(map[string]*MonthlyCost)

	// Charges by month
	cRows, _ := s.pool.Query(ctx, `
		SELECT TO_CHAR(date, 'YYYY-MM') AS m, COALESCE(SUM(cost), 0)
		FROM charge_logs
		WHERE vehicle_id = $1
		GROUP BY m;
	`, vehicleID)
	if cRows != nil {
		for cRows.Next() {
			var m string
			var val float64
			if err := cRows.Scan(&m, &val); err == nil {
				if _, exists := monthlyMap[m]; !exists {
					monthlyMap[m] = &MonthlyCost{Month: m}
				}
				monthlyMap[m].Energy = math.Round(val*100) / 100
			}
		}
		cRows.Close()
	}

	// Tolls by month
	tRows, _ := s.pool.Query(ctx, `
		SELECT TO_CHAR(date, 'YYYY-MM') AS m, COALESCE(SUM(amount), 0)
		FROM drive_expenses
		WHERE vehicle_id = $1
		GROUP BY m;
	`, vehicleID)
	if tRows != nil {
		for tRows.Next() {
			var m string
			var val float64
			if err := tRows.Scan(&m, &val); err == nil {
				if _, exists := monthlyMap[m]; !exists {
					monthlyMap[m] = &MonthlyCost{Month: m}
				}
				monthlyMap[m].Tolls = math.Round(val*100) / 100
			}
		}
		tRows.Close()
	}

	// Maintenance by month
	mRows, _ := s.pool.Query(ctx, `
		SELECT TO_CHAR(date, 'YYYY-MM') AS m, COALESCE(SUM(amount), 0)
		FROM maintenance_expenses
		WHERE vehicle_id = $1
		GROUP BY m;
	`, vehicleID)
	if mRows != nil {
		for mRows.Next() {
			var m string
			var val float64
			if err := mRows.Scan(&m, &val); err == nil {
				if _, exists := monthlyMap[m]; !exists {
					monthlyMap[m] = &MonthlyCost{Month: m}
				}
				monthlyMap[m].Maintenance = math.Round(val*100) / 100
			}
		}
		mRows.Close()
	}

	// Tires by month
	tireRows, _ := s.pool.Query(ctx, `
		SELECT TO_CHAR(purchase_date, 'YYYY-MM') AS m, COALESCE(SUM(purchase_price), 0)
		FROM tires
		WHERE vehicle_id = $1
		GROUP BY m;
	`, vehicleID)
	if tireRows != nil {
		for tireRows.Next() {
			var m string
			var val float64
			if err := tireRows.Scan(&m, &val); err == nil {
				if _, exists := monthlyMap[m]; !exists {
					monthlyMap[m] = &MonthlyCost{Month: m}
				}
				monthlyMap[m].Tires = math.Round(val*100) / 100
			}
		}
		tireRows.Close()
	}

	// Distance driven by month
	dRows, _ := s.pool.Query(ctx, `
		SELECT TO_CHAR(start_time, 'YYYY-MM') AS m, COALESCE(SUM(distance_km), 0)
		FROM drives
		WHERE vehicle_id = $1
		GROUP BY m;
	`, vehicleID)
	if dRows != nil {
		for dRows.Next() {
			var m string
			var val float64
			if err := dRows.Scan(&m, &val); err == nil {
				if _, exists := monthlyMap[m]; !exists {
					monthlyMap[m] = &MonthlyCost{Month: m}
				}
				monthlyMap[m].DistanceKm = math.Round(val*10) / 10
			}
		}
		dRows.Close()
	}

	// Sort monthly costs chronologically and calculate monthly CostPerKm
	var monthlyCosts []MonthlyCost
	for _, mc := range monthlyMap {
		mc.Total = math.Round((mc.Energy+mc.Tolls+mc.Maintenance+mc.Tires)*100) / 100
		if mc.DistanceKm > 0 {
			mc.CostPerKm = math.Round((mc.Total/mc.DistanceKm)*1000) / 1000
		}
		monthlyCosts = append(monthlyCosts, *mc)
	}
	sort.Slice(monthlyCosts, func(i, j int) bool {
		return monthlyCosts[i].Month < monthlyCosts[j].Month
	})

	// If no data, provide current month
	if len(monthlyCosts) == 0 {
		currentMonth := time.Now().Format("2006-01")
		monthlyCosts = append(monthlyCosts, MonthlyCost{
			Month: currentMonth,
		})
	}

	return &TCOSummary{
		TotalDistanceKm:      math.Round(totalDistance*10) / 10,
		TotalCost:            math.Round(totalCost*100) / 100,
		TotalCostPerKm:       math.Round(costPerKm*1000) / 1000,
		EnergyCost:           math.Round(totalEnergyCost*100) / 100,
		EnergyCostPerKm:      math.Round(energyPerKm*1000) / 1000,
		TotalKwhAdded:        math.Round(totalKwhAdded*10) / 10,
		AvgCostPerKwh:        math.Round(avgCostKwh*1000) / 1000,
		TollsCost:            math.Round(totalTollsCost*100) / 100,
		TollsCostPerKm:       math.Round(tollsPerKm*1000) / 1000,
		TiresCost:            math.Round(totalTiresCost*100) / 100,
		TiresCostPerKm:       math.Round(tiresPerKm*1000) / 1000,
		MaintenanceCost:      math.Round(totalMaintenanceCost*100) / 100,
		MaintenanceCostPerKm: math.Round(maintPerKm*1000) / 1000,
		TagBreakdown:         tagBreakdown,
		MonthlyCosts:         monthlyCosts,
	}, nil
}
