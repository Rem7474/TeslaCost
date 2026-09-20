package services

import (
	"testing"
	"time"
)

func TestCalculateMaintenanceAmortization(t *testing.T) {
	loc := time.UTC
	marchDate := time.Date(2024, 3, 15, 10, 0, 0, 0, loc)
	aprilDate := time.Date(2024, 4, 10, 10, 0, 0, 0, loc)
	mayDate := time.Date(2024, 5, 20, 10, 0, 0, 0, loc)
	septDate := time.Date(2024, 9, 1, 10, 0, 0, 0, loc)

	monthlyDistances := map[string]float64{
		"2024-03": 1000,
		"2024-04": 2000,
		"2024-05": 1500,
		"2024-06": 1000,
		"2024-07": 1000,
		"2024-08": 1000,
		"2024-09": 1000,
	}

	t.Run("Distance based amortization with closure by next revision", func(t *testing.T) {
		rev1ID := "rev-1"
		rev2ID := "rev-2"
		items := []*MaintenanceAmortItem{
			{
				ID:             rev1ID,
				Date:           marchDate,
				Month:          "2024-03",
				AmountEur:      50000, // 500.00 €
				Mode:           "DISTANCE",
				CoverageKm:     50000, // Rate = 1.00 €-cent / km
				CoverageMonths: 24,
			},
			{
				ID:             "wipers",
				Date:           aprilDate,
				Month:          "2024-04",
				AmountEur:      4000,   // 40.00 €
				Mode:           "NONE", // Immediate: does NOT close rev1
				CoverageKm:     0,
				CoverageMonths: 0,
			},
			{
				ID:                  rev2ID,
				Date:                septDate,
				Month:               "2024-09",
				AmountEur:           60000, // 600.00 €
				Mode:                "DISTANCE",
				CoverageKm:          50000, // Rate = 1.20 €-cent / km
				CoverageMonths:      24,
				ClosesMaintenanceID: &rev1ID, // Closes rev1 starting at 2024-09!
			},
		}

		res := calculateMaintenanceAmortization(items, monthlyDistances, "2024-09")

		// 2024-03: 1000 km * 1 cent/km = 1000 cents (10.00 €)
		if res["2024-03"] != 1000 {
			t.Errorf("expected 1000 cents in 2024-03, got %d", res["2024-03"])
		}

		// 2024-04: 2000 km * 1 cent/km = 2000 cents + wipers (4000 cents) = 6000 cents (60.00 €)
		if res["2024-04"] != 6000 {
			t.Errorf("expected 6000 cents in 2024-04, got %d", res["2024-04"])
		}

		// 2024-05: 1500 km * 1 cent/km = 1500 cents (15.00 €)
		if res["2024-05"] != 1500 {
			t.Errorf("expected 1500 cents in 2024-05, got %d", res["2024-05"])
		}

		// 2024-09: rev1 closed! Only rev2 amortizes: 1000 km * 1.20 cents/km = 1200 cents (12.00 €)
		if res["2024-09"] != 1200 {
			t.Errorf("expected 1200 cents in 2024-09, got %d", res["2024-09"])
		}
	})

	t.Run("Duration based amortization (e.g. 12 months)", func(t *testing.T) {
		items := []*MaintenanceAmortItem{
			{
				ID:             "ct",
				Date:           marchDate,
				Month:          "2024-03",
				AmountEur:      12000, // 120.00 €
				Mode:           "DURATION",
				CoverageMonths: 12, // 10.00 € / month for 12 months
			},
		}

		res := calculateMaintenanceAmortization(items, monthlyDistances, "2024-09")

		// 2024-03: 10.00 € = 1000 cents
		if res["2024-03"] != 1000 {
			t.Errorf("expected 1000 cents in 2024-03, got %d", res["2024-03"])
		}
		// 2024-04: 1000 cents
		if res["2024-04"] != 1000 {
			t.Errorf("expected 1000 cents in 2024-04, got %d", res["2024-04"])
		}
	})

	t.Run("Hybrid amortization (faster of distance or duration)", func(t *testing.T) {
		// 240 € over 24 months (10 € / month) or 24,000 km (1.00 cent / km)
		items := []*MaintenanceAmortItem{
			{
				ID:             "hybrid-service",
				Date:           mayDate,
				Month:          "2024-05",
				AmountEur:      24000,
				Mode:           "HYBRID",
				CoverageKm:     24000,
				CoverageMonths: 24,
			},
		}

		// In 2024-05: 1500 km -> fracDist = 1500/24000 = 0.0625 (15.00 €).
		// fracTime = 1/24 = 0.0416 (10.00 €).
		// Max fraction is distance (0.0625) -> target 1500 cents (15.00 €).
		res := calculateMaintenanceAmortization(items, monthlyDistances, "2024-09")
		if res["2024-05"] != 1500 {
			t.Errorf("expected 1500 cents in 2024-05 for hybrid, got %d", res["2024-05"])
		}
	})

	t.Run("No retroactive impact on prior months", func(t *testing.T) {
		items := []*MaintenanceAmortItem{
			{
				ID:         "future-rev",
				Date:       mayDate,
				Month:      "2024-05",
				AmountEur:  50000,
				Mode:       "DISTANCE",
				CoverageKm: 50000,
			},
		}
		res := calculateMaintenanceAmortization(items, monthlyDistances, "2024-09")
		if res["2024-03"] != 0 || res["2024-04"] != 0 {
			t.Errorf("expected 0 amortization in months prior to expense date, got 2024-03: %d, 2024-04: %d", res["2024-03"], res["2024-04"])
		}
	})
}
