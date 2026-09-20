package services

import "time"

// allocateMissingKmByMonth distributes missingKm across the calendar months overlapping
// [t1, t2), weighted by the duration of overlap each month has within that interval.
func allocateMissingKmByMonth(t1, t2 time.Time, missingKm float64) map[string]float64 {
	result := make(map[string]float64)

	totalHours := t2.Sub(t1).Hours()
	if totalHours <= 0 {
		return result
	}

	loc := t1.Location()
	cur := time.Date(t1.Year(), t1.Month(), 1, 0, 0, 0, 0, loc)
	endMonth := time.Date(t2.Year(), t2.Month(), 1, 0, 0, 0, 0, loc)

	for !cur.After(endMonth) {
		nextMonth := cur.AddDate(0, 1, 0)
		monthStr := cur.Format("2006-01")

		overlapStart := t1
		if cur.After(overlapStart) {
			overlapStart = cur
		}
		overlapEnd := t2
		if nextMonth.Before(overlapEnd) {
			overlapEnd = nextMonth
		}

		if overlapEnd.After(overlapStart) {
			overlapHours := overlapEnd.Sub(overlapStart).Hours()
			ratio := overlapHours / totalHours
			result[monthStr] += missingKm * ratio
		}

		cur = nextMonth
	}

	return result
}

// allocateSmoothingForInterval allocates missingKm across months for interval [t1, t2).
// It separates total distance smoothing from smoothing before tracking started.
// If firstTrackingTime is provided, only the portion of the interval occurring strictly
// before firstTrackingTime is counted towards the energy estimation.
// Any portion occurring on or after firstTrackingTime is in-TeslaMate smoothing (e.g. GPS
// drift) where real TeslaMate charges already cover energy consumption.
func allocateSmoothingForInterval(t1, t2 time.Time, missingKm float64, firstTrackingTime *time.Time) (smoothed map[string]float64, preTm map[string]float64) {
	smoothed = allocateMissingKmByMonth(t1, t2, missingKm)
	preTm = make(map[string]float64)

	if missingKm <= 0 {
		return smoothed, preTm
	}

	if firstTrackingTime == nil {
		// No TeslaMate data recorded at all: entire interval predates tracking
		preTm = allocateMissingKmByMonth(t1, t2, missingKm)
		return smoothed, preTm
	}

	if !t2.After(*firstTrackingTime) {
		// Entire interval ends on or before tracking started
		preTm = allocateMissingKmByMonth(t1, t2, missingKm)
		return smoothed, preTm
	}

	if !t1.Before(*firstTrackingTime) {
		// Entire interval starts on or after TeslaMate tracking started
		// All missing km are in-TeslaMate smoothing: preTm is empty
		return smoothed, preTm
	}

	// Interval crosses the boundary: prorate only the duration before tracking
	totalHours := t2.Sub(t1).Hours()
	preHours := firstTrackingTime.Sub(t1).Hours()
	if totalHours > 0 && preHours > 0 {
		preMissingKm := missingKm * (preHours / totalHours)
		preTm = allocateMissingKmByMonth(t1, *firstTrackingTime, preMissingKm)
	}

	return smoothed, preTm
}
