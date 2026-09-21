package services

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

var tripBase = time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)

func tripDrive(id string, startMin, endMin int, km float64) models.TripCandidateDrive {
	return models.TripCandidateDrive{ID: id, Start: tripBase.Add(time.Duration(startMin) * time.Minute), End: tripBase.Add(time.Duration(endMin) * time.Minute), DistanceKm: km}
}

func TestDetectTripSuggestionsShortPause(t *testing.T) {
	got := DetectTripSuggestions([]models.TripCandidateDrive{
		tripDrive("a", 0, 60, 80), tripDrive("b", 63, 120, 70), tripDrive("c", 200, 230, 20),
	}, nil)
	if len(got) != 1 || len(got[0].DriveIDs) != 2 || got[0].DriveIDs[0] != "a" || got[0].DriveIDs[1] != "b" {
		t.Fatalf("a and b (3 min pause) form one trip, c (80 min later, no charge) does not: %+v", got)
	}
	if got[0].DistanceKm != 150 || got[0].Reason != models.TripReasonPause {
		t.Fatalf("unexpected suggestion %+v", got[0])
	}
}

func TestDetectTripSuggestionsPauseBoundary(t *testing.T) {
	if got := DetectTripSuggestions([]models.TripCandidateDrive{tripDrive("a", 0, 60, 10), tripDrive("b", 65, 90, 10)}, nil); len(got) != 0 {
		t.Fatalf("a pause of exactly 5 minutes is not shorter than the limit: %+v", got)
	}
}

func TestDetectTripSuggestionsChargeKeepsTripTogether(t *testing.T) {
	drives := []models.TripCandidateDrive{tripDrive("a", 0, 120, 200), tripDrive("b", 165, 240, 150)}
	charge := []models.ChargeWindow{{Start: tripBase.Add(125 * time.Minute), End: tripBase.Add(160 * time.Minute)}}
	got := DetectTripSuggestions(drives, charge)
	if len(got) != 1 || got[0].Reason != models.TripReasonCharge {
		t.Fatalf("a 45 min stop with a charge is the same trip: %+v", got)
	}
	if got := DetectTripSuggestions(drives, nil); len(got) != 0 {
		t.Fatalf("the same stop without a charge is not: %+v", got)
	}
}

func TestDetectTripSuggestionsChargeStopCap(t *testing.T) {
	drives := []models.TripCandidateDrive{tripDrive("a", 0, 60, 30), tripDrive("b", 60+4*60, 400, 30)}
	charge := []models.ChargeWindow{{Start: tripBase.Add(70 * time.Minute), End: tripBase.Add(200 * time.Minute)}}
	if got := DetectTripSuggestions(drives, charge); len(got) != 0 {
		t.Fatalf("a 4 h stop is beyond the charge stop limit (overnight or workday charge): %+v", got)
	}
}

func TestDetectTripSuggestionsIgnoresGroupedDrives(t *testing.T) {
	a, b := tripDrive("a", 0, 60, 10), tripDrive("b", 62, 90, 10)
	b.Grouped = true
	if got := DetectTripSuggestions([]models.TripCandidateDrive{a, b}, nil); len(got) != 0 {
		t.Fatalf("a chain touching a grouped drive is left to the user: %+v", got)
	}
}

func TestDetectTripSuggestionsUnsortedInputAndOrder(t *testing.T) {
	got := DetectTripSuggestions([]models.TripCandidateDrive{
		tripDrive("d", 1000, 1030, 5), tripDrive("b", 61, 90, 5), tripDrive("a", 0, 60, 5), tripDrive("c", 1033, 1100, 5),
	}, nil)
	if len(got) != 2 || len(got[0].DriveIDs) != 2 || got[0].DriveIDs[0] != "d" || got[0].DriveIDs[1] != "c" || got[1].DriveIDs[0] != "a" || got[1].DriveIDs[1] != "b" {
		t.Fatalf("expected the recent chain first, each in chronological order: %+v", got)
	}
}
