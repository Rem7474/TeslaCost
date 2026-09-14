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
