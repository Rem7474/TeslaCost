package services

import "testing"

func TestCompletenessScore(t *testing.T) {
	full := completenessInputs{
		kwhAdded: 500, kwhPriced: 500, highwayDrives: 10, unqualifiedDrives: 0,
		trackedKm: 10000, basisKm: 10000, insurancePresent: true, acquisitionComplete: true,
		pricedEntries: 40, drivesWithOdometer: 200,
	}
	if score, dims := completenessScore(full); score != 100 || len(dims) != 7 {
		t.Fatalf("expected 100 %% over 7 dimensions, got %d (%d)", score, len(dims))
	}

	partial := full
	partial.kwhPriced = 250          // energy 50 % → −15 pts
	partial.unqualifiedDrives = 5    // tolls 50 % → −7.5 pts
	partial.insurancePresent = false // −10 pts
	score, dims := completenessScore(partial)
	if score != 68 {
		t.Fatalf("expected 67.5 rounded to 68, got %d", score)
	}
	if dims[0].Key != "energy" || dims[0].ScorePct != 50 {
		t.Fatalf("unexpected energy dimension: %+v", dims[0])
	}

	noDistance := full
	noDistance.basisKm, noDistance.trackedKm = 0, 0
	if score, _ := completenessScore(noDistance); score != 80 {
		t.Fatalf("no kilometers at all must cost the distance weight, got %d", score)
	}
}
