package tolldata

import (
	"math"
	"testing"
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

func TestDetectCrossings_MultipleSegmentsOnOneDrive(t *testing.T) {
	// 11 points, ~222m apart, spanning ~2.2km along the equator.
	trace := buildStraightTrace(11, 0.002)

	stations := []Station{
		{Name: "OPEN1", Type: "open", Operator: "OP1", Lat: 0, Lon: 0.006},
		{Name: "ENTRY_A", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.010},
		{Name: "EXIT_A", Type: "close", Operator: "OP1", Lat: 0, Lon: 0.014},
		{Name: "LONE_CLOSE", Type: "close", Operator: "OP2", Lat: 0, Lon: 0.020},
		{Name: "FAR_STATION", Type: "open", Operator: "OP3", Lat: 5, Lon: 5},
	}
	networkByStation := map[string]string{
		"ENTRY_A":    "component_1",
		"EXIT_A":     "component_1",
		"LONE_CLOSE": "component_2",
	}

	matches := DetectCrossings(trace, stations, DefaultThresholdMeters)

	if len(matches) != 4 {
		names := make([]string, len(matches))
		for i, m := range matches {
			names[i] = m.Station.Name
		}
		t.Fatalf("expected 4 matches (FAR_STATION excluded), got %d: %v", len(matches), names)
	}

	wantOrder := []string{"OPEN1", "ENTRY_A", "EXIT_A", "LONE_CLOSE"}
	for i, want := range wantOrder {
		if matches[i].Station.Name != want {
			t.Errorf("match[%d]: expected %q, got %q", i, want, matches[i].Station.Name)
		}
	}

	segments := BuildSegments(matches, networkByStation)
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d: %+v", len(segments), segments)
	}

	if segments[0].Type != "open" || segments[0].Entry != "OPEN1" || segments[0].Exit != nil {
		t.Errorf("segment[0]: expected open barrier OPEN1, got %+v", segments[0])
	}

	if segments[1].Type != "close" || segments[1].Entry != "ENTRY_A" ||
		segments[1].Exit == nil || *segments[1].Exit != "EXIT_A" || segments[1].Network != "component_1" {
		t.Errorf("segment[1]: expected closed pair ENTRY_A -> EXIT_A on component_1, got %+v", segments[1])
	}

	if segments[2].Type != "close" || segments[2].Entry != "LONE_CLOSE" || segments[2].Exit != nil {
		t.Errorf("segment[2]: expected incomplete closed entry LONE_CLOSE with no exit, got %+v", segments[2])
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
