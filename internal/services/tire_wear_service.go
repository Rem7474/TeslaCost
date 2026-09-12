package services

import (
	"context"
	"math"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
)

// TireWearStats provides detailed wear analytics for a tire.
type TireWearStats struct {
	Tire                 models.Tire               `json:"tire"`
	CurrentDepthMm       float64                   `json:"current_depth_mm"`
	InitialDepthMm       float64                   `json:"initial_depth_mm"`
	MinLegalDepthMm      float64                   `json:"min_legal_depth_mm"`
	UsableDepthMm        float64                   `json:"usable_depth_mm"`
	RemainingDepthMm     float64                   `json:"remaining_depth_mm"`
	WearPercentage       float64                   `json:"wear_percentage"`
	DistanceTraveledKm   float64                   `json:"distance_traveled_km"`
	TotalDistanceKm      float64                   `json:"total_distance_km"`
	EstimatedLifespanKm  int                       `json:"estimated_lifespan_km"`
	LifeProgressPct      float64                   `json:"life_progress_pct"`
	CostPerKm            float64                   `json:"cost_per_km"`
	WearRatePer10kKm     float64                   `json:"wear_rate_per_10k_km"`
	EstimatedRemainingKm float64                   `json:"estimated_remaining_km"`
	Condition            string                    `json:"condition"` // "GOOD", "WARNING", "CRITICAL"
	LogsCount            int                       `json:"logs_count"`
	Sessions             []models.TireMountSession `json:"sessions"`
}

// TireWearService calculates wear projections and stats for tires.
type TireWearService struct {
	repo *database.Repository
}

// NewTireWearService creates a new TireWearService.
func NewTireWearService(repo *database.Repository) *TireWearService {
	return &TireWearService{repo: repo}
}

// CalculateTireWear computes wear metrics based on depth logs and mount sessions.
func (s *TireWearService) CalculateTireWear(ctx context.Context, tire *models.Tire, vehicleCurrentOdometer float64) (*TireWearStats, error) {
	logs, err := s.repo.ListTireLogs(ctx, tire.ID)
	if err != nil {
		return nil, err
	}

	sessions, _ := s.repo.ListTireMountSessions(ctx, tire.ID)

	initialDepth := tire.InitialDepthMm
	if initialDepth <= 0 {
		initialDepth = 8.0 // standard default for passenger car tire
	}

	minLegal := tire.MinLegalDepthMm
	if minLegal <= 0 {
		minLegal = 1.6 // EU legal minimum
	}

	usableDepth := math.Max(0.1, initialDepth-minLegal)
	currentDepth := initialDepth
	distanceTraveled := 0.0

	if len(logs) > 0 {
		// Latest log is first due to ORDER BY date DESC
		latestLog := logs[0]
		currentDepth = latestLog.DepthMm

		// Oldest log is last
		oldestLog := logs[len(logs)-1]
		if len(logs) >= 2 {
			distanceTraveled = math.Max(0, latestLog.Odometer-oldestLog.Odometer)
		} else if vehicleCurrentOdometer > latestLog.Odometer {
			distanceTraveled = vehicleCurrentOdometer - latestLog.Odometer
		}
	}

	// Calculate lifetime distance
	currentRunKm := 0.0
	isMounted := tire.CurrentPosition != models.TirePosStorage && tire.CurrentPosition != models.TirePosDisposed
	if isMounted && tire.MountedOdometer != nil && vehicleCurrentOdometer > *tire.MountedOdometer {
		currentRunKm = vehicleCurrentOdometer - *tire.MountedOdometer
	}
	totalDistance := tire.AccumulatedDistanceKm + currentRunKm

	// Update active session distance in-memory for display
	for i := range sessions {
		if sessions[i].DismountedDate == nil && isMounted {
			sessions[i].DistanceKm = math.Round(currentRunKm*10) / 10
		}
	}

	lifespan := tire.EstimatedLifespanKm
	if lifespan <= 0 {
		lifespan = 40000
	}
	lifeProgressPct := math.Min(100.0, math.Round((totalDistance/float64(lifespan))*1000)/10)
	costPerKm := math.Round((tire.PurchasePrice/float64(lifespan))*10000) / 10000

	wornDepth := math.Max(0, initialDepth-currentDepth)
	remainingDepth := math.Max(0, currentDepth-minLegal)
	wearPct := math.Min(100.0, (wornDepth/usableDepth)*100.0)

	wearRatePer10k := 0.0
	estimatedRemainingKm := 0.0

	if distanceTraveled > 500 && wornDepth > 0.05 {
		wearRatePer10k = (wornDepth / distanceTraveled) * 10000.0
		if wearRatePer10k > 0 {
			estimatedRemainingKm = (remainingDepth / wearRatePer10k) * 10000.0
		}
	} else {
		// Fallback estimation using EV average wear rate (approx 1.2 mm / 10,000 km)
		wearRatePer10k = 1.2
		estimatedRemainingKm = (remainingDepth / 1.2) * 10000.0
	}

	condition := "GOOD"
	if currentDepth <= 2.5 {
		condition = "CRITICAL"
	} else if currentDepth <= 4.0 {
		condition = "WARNING"
	}

	return &TireWearStats{
		Tire:                 *tire,
		CurrentDepthMm:       math.Round(currentDepth*10) / 10,
		InitialDepthMm:       initialDepth,
		MinLegalDepthMm:      minLegal,
		UsableDepthMm:        math.Round(usableDepth*10) / 10,
		RemainingDepthMm:     math.Round(remainingDepth*10) / 10,
		WearPercentage:       math.Round(wearPct*10) / 10,
		DistanceTraveledKm:   math.Round(distanceTraveled),
		TotalDistanceKm:      math.Round(totalDistance*10) / 10,
		EstimatedLifespanKm:  lifespan,
		LifeProgressPct:      lifeProgressPct,
		CostPerKm:            costPerKm,
		WearRatePer10kKm:     math.Round(wearRatePer10k*100) / 100,
		EstimatedRemainingKm: math.Round(estimatedRemainingKm),
		Condition:            condition,
		LogsCount:            len(logs),
		Sessions:             sessions,
	}, nil
}

