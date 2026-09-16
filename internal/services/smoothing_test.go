package services

import (
	"math"
	"testing"
	"time"
)

func TestMileageSmoothingDistribution(t *testing.T) {
	loc := time.UTC

	// Scenario:
	// A vehicle was bought on 2023-01-01 at 20,000 km.
	// A manual checkpoint is entered on 2023-04-01 at 29,000 km.
	// Delta odometer = 9,000 km over 90 days (Jan: 31, Feb: 28, Mar: 31).
	t1 := time.Date(2023, 1, 1, 0, 0, 0, 0, loc)
	t2 := time.Date(2023, 4, 1, 0, 0, 0, 0, loc)
	deltaOdo := 9000.0
	trackedKm := 0.0
	missingKm := deltaOdo - trackedKm

	smoothedByMonth := allocateMissingKmByMonth(t1, t2, missingKm)

	// January: 31 days / 90 days * 9000 = 3100 km
	jan := smoothedByMonth["2023-01"]
	if math.Abs(jan-3100.0) > 0.01 {
		t.Errorf("expected January to have 3100 km, got %f", jan)
	}

	// February: 28 days / 90 days * 9000 = 2800 km
	feb := smoothedByMonth["2023-02"]
	if math.Abs(feb-2800.0) > 0.01 {
		t.Errorf("expected February to have 2800 km, got %f", feb)
	}

	// March: 31 days / 90 days * 9000 = 3100 km
	mar := smoothedByMonth["2023-03"]
	if math.Abs(mar-3100.0) > 0.01 {
		t.Errorf("expected March to have 3100 km, got %f", mar)
	}

	// April: starts at 2023-04-01 00:00, no overlap duration
	apr := smoothedByMonth["2023-04"]
	if apr > 0.001 {
		t.Errorf("expected April to have 0 smoothed km, got %f", apr)
	}

	totalSmoothed := jan + feb + mar + apr
	if math.Abs(totalSmoothed-9000.0) > 0.01 {
		t.Errorf("expected total smoothed distance to be 9000 km, got %f", totalSmoothed)
	}
}

func TestMileageSmoothingWithPartialDrives(t *testing.T) {
	loc := time.UTC

	// Between 2023-05-01 and 2023-06-01:
	// Odometer delta = 3,000 km (31 days).
	// TeslaMate tracked 1,000 km of drives.
	// Missing km = 2,000 km.
	t1 := time.Date(2023, 5, 1, 0, 0, 0, 0, loc)
	t2 := time.Date(2023, 6, 1, 0, 0, 0, 0, loc)
	deltaOdo := 3000.0
	trackedKm := 1000.0
	missingKm := deltaOdo - trackedKm

	smoothedByMonth := allocateMissingKmByMonth(t1, t2, missingKm)

	may := smoothedByMonth["2023-05"]
	if math.Abs(may-2000.0) > 0.01 {
		t.Errorf("expected May to have 2000 smoothed km, got %f", may)
	}

	totalEffectiveMay := trackedKm + may
	if math.Abs(totalEffectiveMay-3000.0) > 0.01 {
		t.Errorf("expected total effective May distance to equal deltaOdo (3000 km), got %f", totalEffectiveMay)
	}
}

func TestPreTeslaMateEnergyCalculation(t *testing.T) {
	// 9000 km smoothed over Jan, Feb, Mar
	smoothedKm := 9000.0
	kwh100km := 16.5
	eurPerKwh := 0.22

	totalKwh := math.Round(smoothedKm*(kwh100km/100.0)*10) / 10
	expectedKwh := 1485.0
	if math.Abs(totalKwh-expectedKwh) > 0.01 {
		t.Errorf("expected total kWh to be %f, got %f", expectedKwh, totalKwh)
	}

	totalCostEur := totalKwh * eurPerKwh
	expectedCost := 326.70
	if math.Abs(totalCostEur-expectedCost) > 0.01 {
		t.Errorf("expected total cost to be %f €, got %f €", expectedCost, totalCostEur)
	}
}

func TestAllocateSmoothingForInterval_PostTeslaMate_NoPreTeslaMateEnergy(t *testing.T) {
	loc := time.UTC
	// TeslaMate started tracking on 2023-01-01
	firstTrackingTime := time.Date(2023, 1, 1, 0, 0, 0, 0, loc)

	// An interval between 2024-05-01 and 2024-06-01 (long after TeslaMate started)
	t1 := time.Date(2024, 5, 1, 0, 0, 0, 0, loc)
	t2 := time.Date(2024, 6, 1, 0, 0, 0, 0, loc)
	missingKm := 36.6 // minor GPS drift smoothing

	smoothed, preTm := allocateSmoothingForInterval(t1, t2, missingKm, &firstTrackingTime)

	// Distance smoothing should still apply for May 2024
	if math.Abs(smoothed["2024-05"]-36.6) > 0.01 {
		t.Errorf("expected 36.6 smoothed km in 2024-05, got %f", smoothed["2024-05"])
	}

	// But pre-TeslaMate km MUST BE ZERO so no synthetic energy is added when real data is available!
	if len(preTm) != 0 || preTm["2024-05"] > 0 {
		t.Errorf("expected 0 pre-TeslaMate km for post-tracking interval, got %v", preTm)
	}
}

func TestAllocateSmoothingForInterval_PreTeslaMate_FullPreTeslaMateEnergy(t *testing.T) {
	loc := time.UTC
	// TeslaMate started tracking on 2023-06-01
	firstTrackingTime := time.Date(2023, 6, 1, 0, 0, 0, 0, loc)

	// An interval between 2022-01-01 and 2022-03-01 (before TeslaMate started)
	t1 := time.Date(2022, 1, 1, 0, 0, 0, 0, loc)
	t2 := time.Date(2022, 3, 1, 0, 0, 0, 0, loc)
	missingKm := 2000.0

	smoothed, preTm := allocateSmoothingForInterval(t1, t2, missingKm, &firstTrackingTime)

	// Both smoothed and preTm should have the missing km allocated across Jan & Feb 2022
	var totalPreTm float64
	for _, km := range preTm {
		totalPreTm += km
	}
	if math.Abs(totalPreTm-2000.0) > 0.01 {
		t.Errorf("expected 2000 pre-TeslaMate km, got %f", totalPreTm)
	}
	if math.Abs(smoothed["2022-01"]-preTm["2022-01"]) > 0.01 {
		t.Errorf("expected smoothed and preTm to match for pre-TeslaMate interval")
	}
}

func TestAllocateSmoothingForInterval_SpanningBoundary(t *testing.T) {
	loc := time.UTC
	// TeslaMate started tracking on 2023-05-15 12:00:00 (mid-May)
	firstTrackingTime := time.Date(2023, 5, 15, 12, 0, 0, 0, loc)

	// Interval starts 2023-05-01 00:00 and ends 2023-05-30 00:00 (29 days)
	t1 := time.Date(2023, 5, 1, 0, 0, 0, 0, loc)
	t2 := time.Date(2023, 5, 30, 0, 0, 0, 0, loc)
	missingKm := 290.0 // 10 km/day

	smoothed, preTm := allocateSmoothingForInterval(t1, t2, missingKm, &firstTrackingTime)

	// Full interval distance smoothed
	if math.Abs(smoothed["2023-05"]-290.0) > 0.01 {
		t.Errorf("expected 290 smoothed km, got %f", smoothed["2023-05"])
	}

	// Pre-TeslaMate portion is exactly 14.5 days out of 29 days = 50% = 145 km
	if math.Abs(preTm["2023-05"]-145.0) > 0.01 {
		t.Errorf("expected 145 pre-TeslaMate km, got %f", preTm["2023-05"])
	}
}

func TestAllocateSmoothingForInterval_NoTeslaMateData(t *testing.T) {
	loc := time.UTC
	t1 := time.Date(2023, 1, 1, 0, 0, 0, 0, loc)
	t2 := time.Date(2023, 2, 1, 0, 0, 0, 0, loc)
	missingKm := 500.0

	smoothed, preTm := allocateSmoothingForInterval(t1, t2, missingKm, nil)

	if math.Abs(smoothed["2023-01"]-500.0) > 0.01 {
		t.Errorf("expected 500 smoothed km, got %f", smoothed["2023-01"])
	}
	if math.Abs(preTm["2023-01"]-500.0) > 0.01 {
		t.Errorf("expected 500 preTm km, got %f", preTm["2023-01"])
	}
}

