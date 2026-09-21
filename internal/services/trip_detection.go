package services

import (
	"math"
	"sort"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

const (
	// Two drives separated by less than this are the same trip.
	TripPauseMax = 5 * time.Minute
	// A longer stop still belongs to the trip when the car was charging during it (road trip), up to this duration.
	TripChargeStopMax = 3 * time.Hour
	// Charge timestamps and drive ends are recorded independently, so a charge may start slightly before a drive ends.
	tripChargeSlack = 5 * time.Minute
)

func chargedBetween(charges []models.ChargeWindow, from, to time.Time) bool {
	for _, c := range charges {
		end := c.End
		if end.Before(c.Start) {
			end = c.Start
		}
		if c.Start.Before(to.Add(tripChargeSlack)) && end.After(from.Add(-tripChargeSlack)) {
			return true
		}
	}
	return false
}

// DetectTripSuggestions groups consecutive drives into probable trips: a stop shorter than TripPauseMax, or a
// stop up to TripChargeStopMax during which the car was charging, keeps the drives in the same trip. Only chains
// of at least two drives that are all still ungrouped are suggested, most recent first.
func DetectTripSuggestions(drives []models.TripCandidateDrive, charges []models.ChargeWindow) []models.TripSuggestion {
	sorted := append([]models.TripCandidateDrive(nil), drives...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start.Before(sorted[j].Start) })

	var suggestions []models.TripSuggestion
	flush := func(chain []models.TripCandidateDrive, viaCharge bool) {
		if len(chain) < 2 {
			return
		}
		for _, d := range chain {
			if d.Grouped {
				return
			}
		}
		s := models.TripSuggestion{StartTime: chain[0].Start, EndTime: chain[len(chain)-1].EndOrStart(), Reason: models.TripReasonPause}
		if viaCharge {
			s.Reason = models.TripReasonCharge
		}
		for _, d := range chain {
			s.DriveIDs = append(s.DriveIDs, d.ID)
			s.DistanceKm += d.DistanceKm
		}
		s.DistanceKm = math.Round(s.DistanceKm*10) / 10
		if l := placeLabel(chain[0].StartAddress); l != nil {
			s.StartAddress = *l
		}
		if l := placeLabel(chain[len(chain)-1].EndAddress); l != nil {
			s.EndAddress = *l
		}
		suggestions = append(suggestions, s)
	}

	var chain []models.TripCandidateDrive
	viaCharge := false
	for _, d := range sorted {
		if len(chain) > 0 {
			prev := chain[len(chain)-1]
			gap := d.Start.Sub(prev.EndOrStart())
			switch {
			case gap < TripPauseMax:
			case gap <= TripChargeStopMax && chargedBetween(charges, prev.EndOrStart(), d.Start):
				viaCharge = true
			default:
				flush(chain, viaCharge)
				chain, viaCharge = nil, false
			}
		}
		chain = append(chain, d)
	}
	flush(chain, viaCharge)

	sort.Slice(suggestions, func(i, j int) bool { return suggestions[i].StartTime.After(suggestions[j].StartTime) })
	return suggestions
}
