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
