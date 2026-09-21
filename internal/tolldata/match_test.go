package tolldata

import (
	"math"
	"testing"

	"github.com/teslacost/teslacost/internal/money"
)

// buildStraightTrace returns points along the equator (lat=0) from lon=0 to lon=stepDeg*(n-1),
// where distances are easy to reason about (1 degree ≈ 111km at the equator, both axes).
func buildStraightTrace(n int, stepDeg float64) []LatLon {
	trace := make([]LatLon, n)
	for i := 0; i < n; i++ {
		trace[i] = LatLon{Lat: 0, Lon: stepDeg * float64(i)}
	}
	return trace
}

func TestHaversineMeters_KnownDistance(t *testing.T) {
	a := LatLon{Lat: 0, Lon: 0}
	b := LatLon{Lat: 0, Lon: 1} // 1 degree of longitude at the equator ≈ 111.19 km
	d := haversineMeters(a, b)
	if math.Abs(d-111195) > 500 {
		t.Errorf("expected ~111195m, got %f", d)
	}
}

func TestPointToSegmentMeters_OnSegment(t *testing.T) {
	a := LatLon{Lat: 0, Lon: 0}
	b := LatLon{Lat: 0, Lon: 0.01}
	p := LatLon{Lat: 0, Lon: 0.005} // exactly between a and b
	d := pointToSegmentMeters(p, a, b)
	if d > 1 {
		t.Errorf("expected ~0m for a point on the segment, got %f", d)
	}
}

func TestPointToSegmentMeters_OffToTheSide(t *testing.T) {
	a := LatLon{Lat: 0, Lon: 0}
	b := LatLon{Lat: 0, Lon: 0.01}
	p := LatLon{Lat: 0.001, Lon: 0.005} // ~111m north of the segment's midpoint
	d := pointToSegmentMeters(p, a, b)
	if math.Abs(d-111) > 10 {
		t.Errorf("expected ~111m, got %f", d)
	}
}

func TestMergeCloseVisits_MergesJitter(t *testing.T) {
	trace := buildStraightTrace(5, 0.001) // ~111m steps
	st := Station{Name: "Z"}
	visits := []MatchedStation{
		{Station: st, SegmentIndex: 0},
		{Station: st, SegmentIndex: 1}, // ~111m away, well under MinRevisitGapMeters
	}
	merged := mergeCloseVisits(visits, trace, MinRevisitGapMeters)
	if len(merged) != 1 {
		t.Fatalf("expected jittery visits to merge into 1, got %d", len(merged))
	}
}

func TestMergeCloseVisits_KeepsRealRevisit(t *testing.T) {
	trace := buildStraightTrace(10, 0.005) // ~555m steps
	st := Station{Name: "Z"}
	visits := []MatchedStation{
		{Station: st, SegmentIndex: 0},
		{Station: st, SegmentIndex: 5}, // ~2775m away, well over MinRevisitGapMeters
	}
	merged := mergeCloseVisits(visits, trace, MinRevisitGapMeters)
	if len(merged) != 2 {
		t.Fatalf("expected distinct revisits to stay separate, got %d", len(merged))
	}
}

func TestDetectCrossings_MultipleSegmentsOnOneDrive(t *testing.T) {
	// 21 points, ~111m apart, spanning ~2.2km along the equator.
	trace := buildStraightTrace(21, 0.001)

	stations := []Station{
		{Name: "OPEN1", Type: "open", Operator: "OP1", Lat: 0, Lon: 0.002},
		{Name: "S1", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.004},
		{Name: "S2", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.006},
		{Name: "S3", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.008},
		{Name: "S4", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.010},
		{Name: "S5", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.012},
		{Name: "S6", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.014},
		{Name: "LONE_CLOSE", Type: "close", Operator: "OP2", Lat: 0, Lon: 0.020},
		{Name: "FAR_STATION", Type: "open", Operator: "OP3", Lat: 5, Lon: 5},
	}
	networkByStation := map[string]string{
		"S1": "component_1", "S2": "component_1", "S3": "component_1",
		"S4": "component_1", "S5": "component_1", "S6": "component_1",
		"LONE_CLOSE": "component_2",
	}
	ds := &Dataset{
		NetworkByStation: networkByStation,
		OpenPrice:        map[string]money.Cents{"OPEN1": money.FromFloat(1.10)},
		ClosedPrice: map[string]map[string]money.Cents{
			"S1": {"S6": money.FromFloat(12.30)},
		},
	}

	matches := DetectCrossings(trace, stations, DefaultThresholdMeters)

	if len(matches) != 8 {
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Station.Name
		}
		t.Fatalf("expected 8 matches (FAR_STATION excluded), got %d: %v", len(matches), names)
	}

	wantOrder := []string{"OPEN1", "S1", "S2", "S3", "S4", "S5", "S6", "LONE_CLOSE"}
	for i, want := range wantOrder {
		if matches[i].Station.Name != want {
			t.Errorf("match[%d]: expected %q, got %q", i, want, matches[i].Station.Name)
		}
	}

	// Regression test: 6 consecutive closed stations on the same network must collapse into a
	// single S1->S6 segment, not 3 segments paired two-by-two (the originally reported bug).
	segments := ds.BuildSegments(matches)
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d: %+v", len(segments), segments)
	}

	if segments[0].Type != "open" || segments[0].Entry != "OPEN1" || segments[0].Exit != nil {
		t.Errorf("segment[0]: expected open barrier OPEN1, got %+v", segments[0])
	}
	if segments[0].EstimatedPrice == nil || *segments[0].EstimatedPrice != money.FromFloat(1.10) {
		t.Errorf("segment[0]: expected price 1.10, got %+v", segments[0].EstimatedPrice)
	}

	if segments[1].Type != "close" || segments[1].Entry != "S1" ||
		segments[1].Exit == nil || *segments[1].Exit != "S6" || segments[1].Network != "component_1" {
		t.Errorf("segment[1]: expected closed pair S1 -> S6 on component_1, got %+v", segments[1])
	}
	if segments[1].EstimatedPrice == nil || *segments[1].EstimatedPrice != money.FromFloat(12.30) {
		t.Errorf("segment[1]: expected price 12.30, got %+v", segments[1].EstimatedPrice)
	}

	if segments[2].Type != "close" || segments[2].Entry != "LONE_CLOSE" || segments[2].Exit != nil {
		t.Errorf("segment[2]: expected incomplete closed entry LONE_CLOSE with no exit, got %+v", segments[2])
	}
	if segments[2].EstimatedPrice != nil {
		t.Errorf("segment[2]: expected no price for an incomplete segment, got %+v", segments[2].EstimatedPrice)
	}
}

func TestDetectCrossings_NoStationsNearby(t *testing.T) {
	trace := buildStraightTrace(5, 0.002)
	stations := []Station{
		{Name: "FAR", Type: "open", Lat: 10, Lon: 10},
	}
	matches := DetectCrossings(trace, stations, DefaultThresholdMeters)
	if len(matches) != 0 {
		t.Errorf("expected no matches, got %d", len(matches))
	}
}

func TestDetectCrossings_ShortTraceIsIgnored(t *testing.T) {
	matches := DetectCrossings([]LatLon{{Lat: 0, Lon: 0}}, []Station{{Name: "X", Lat: 0, Lon: 0}}, DefaultThresholdMeters)
	if matches != nil {
		t.Errorf("expected nil for a trace with fewer than 2 points, got %v", matches)
	}
}

func TestBuildSegments_ExitAndReentryAtSameGate(t *testing.T) {
	// A drive enters at W, exits at X (e.g. to drop someone off), takes a > 300m detour off the
	// highway, then re-enters through the very same gate X, and finally exits at Y. Stations are
	// spaced ~1km+ apart so the detour and revisit don't spuriously graze W or Y's thresholds.
	trace := []LatLon{
		{Lat: 0, Lon: 0.0000}, // near W
		{Lat: 0, Lon: 0.0050},
		{Lat: 0, Lon: 0.0100}, // near X (visit 1: exit)
		{Lat: 0.01, Lon: 0.0100},
		{Lat: 0.01, Lon: 0.0110}, // detour, ~1.1km away from X
		{Lat: 0.005, Lon: 0.0100},
		{Lat: 0, Lon: 0.0101}, // near X again (visit 2: re-entry)
		{Lat: 0, Lon: 0.0150},
		{Lat: 0, Lon: 0.0200},
		{Lat: 0, Lon: 0.0300}, // near Y (final exit)
	}
	stations := []Station{
		{Name: "W", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.0000},
		{Name: "X", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.0100},
		{Name: "Y", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.0300},
	}
	networkByStation := map[string]string{"W": "component_1", "X": "component_1", "Y": "component_1"}
	ds := &Dataset{NetworkByStation: networkByStation}

	matches := DetectCrossings(trace, stations, DefaultThresholdMeters)

	var xVisits int
	for _, m := range matches {
		if m.Station.Name == "X" {
			xVisits++
		}
	}
	if xVisits != 2 {
		t.Fatalf("expected 2 distinct visits of X (exit then re-entry), got %d in %+v", xVisits, matches)
	}

	segments := ds.BuildSegments(matches)
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments (W->X, X->Y), got %d: %+v", len(segments), segments)
	}
	if segments[0].Entry != "W" || segments[0].Exit == nil || *segments[0].Exit != "X" {
		t.Errorf("segment[0]: expected W -> X, got %+v", segments[0])
	}
	if segments[1].Entry != "X" || segments[1].Exit == nil || *segments[1].Exit != "Y" {
		t.Errorf("segment[1]: expected X -> Y, got %+v", segments[1])
	}
}

func TestBuildSegments_NetworkChangeSplitsSegments(t *testing.T) {
	matches := []MatchedStation{
		{Station: Station{Name: "A", Type: "close", Operator: "OP1"}, SegmentIndex: 0},
		{Station: Station{Name: "B", Type: "close", Operator: "OP2"}, SegmentIndex: 1},
	}
	ds := &Dataset{NetworkByStation: map[string]string{"A": "component_1", "B": "component_2"}}

	segments := ds.BuildSegments(matches)
	if len(segments) != 2 {
		t.Fatalf("expected 2 segments (different networks), got %d: %+v", len(segments), segments)
	}
	if segments[0].Entry != "A" || segments[0].Exit != nil {
		t.Errorf("segment[0]: expected incomplete entry A, got %+v", segments[0])
	}
	if segments[1].Entry != "B" || segments[1].Exit != nil {
		t.Errorf("segment[1]: expected incomplete entry B, got %+v", segments[1])
	}
}

func TestPriceSegment_MissingClosedPriceLeavesNil(t *testing.T) {
	exit := "B"
	seg := Segment{Type: "close", Entry: "A", Exit: &exit}
	ds := &Dataset{ClosedPrice: map[string]map[string]money.Cents{}}
	ds.priceSegment(&seg)
	if seg.EstimatedPrice != nil {
		t.Errorf("expected no price when the pair is absent from ClosedPrice, got %+v", seg.EstimatedPrice)
	}
}

func TestDefaultThresholdIsFiftyMeters(t *testing.T) {
	// 0.00036° of latitude is about 40 m, 0.00072° about 80 m
	trace := []LatLon{{Lat: 0, Lon: 0}, {Lat: 0, Lon: 0.01}}
	stations := []Station{
		{Name: "NEAR", Lat: 0.00036, Lon: 0.005},
		{Name: "FAR", Lat: 0.00072, Lon: 0.005},
	}
	matches := DetectCrossings(trace, stations, DefaultThresholdMeters)
	if len(matches) != 1 || matches[0].Station.Name != "NEAR" {
		t.Fatalf("a station 40 m from the trace is crossed, one 80 m away is not: %+v", matches)
	}
}
