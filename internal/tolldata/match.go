package tolldata

import (
	"sort"

	"github.com/teslacost/teslacost/internal/money"
)

// DefaultThresholdMeters is the maximum distance between a drive's GPS trace and a toll
// station for that station to be considered crossed.
const DefaultThresholdMeters = 50.0

// MinRevisitGapMeters is the minimum trace distance required between two visits of the same
// station for them to be treated as two distinct passes (e.g. exiting and re-entering through
// the same gate) rather than GPS/threshold jitter around a single pass.
const MinRevisitGapMeters = 300.0

// MatchedStation is a toll station whose distance to the drive's trace fell under the threshold.
type MatchedStation struct {
	Station      Station
	SegmentIndex int // index into the trace segment [i, i+1] where the minimum distance occurred
}

// DetectCrossings returns the stations crossed by trace, in chronological order. A station may
// appear more than once if the trace approaches it, moves away by more than MinRevisitGapMeters,
// then approaches it again (e.g. exiting and re-entering through the same gate).
func DetectCrossings(trace []LatLon, stations []Station, thresholdMeters float64) []MatchedStation {
	if len(trace) < 2 {
		return nil
	}

	minLat, maxLat, minLon, maxLon := boundingBox(trace, thresholdMeters)

	var matches []MatchedStation
	for _, st := range stations {
		if st.Lat < minLat || st.Lat > maxLat || st.Lon < minLon || st.Lon > maxLon {
			continue // coarse pre-filter: station is nowhere near the trace's bounding box
		}

		visits := detectVisits(trace, st, thresholdMeters)
		matches = append(matches, mergeCloseVisits(visits, trace, MinRevisitGapMeters)...)
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].SegmentIndex < matches[j].SegmentIndex
	})
	return matches
}

// detectVisits groups consecutive trace segments under the threshold into visits, keeping the
// closest point of each visit as its representative index.
func detectVisits(trace []LatLon, st Station, thresholdMeters float64) []MatchedStation {
	var visits []MatchedStation
	inVisit := false
	var bestDist float64
	bestIdx := -1

	for i := 0; i < len(trace)-1; i++ {
		d := pointToSegmentMeters(LatLon{Lat: st.Lat, Lon: st.Lon}, trace[i], trace[i+1])
		switch {
		case d <= thresholdMeters && !inVisit:
			inVisit, bestDist, bestIdx = true, d, i
		case d <= thresholdMeters && d < bestDist:
			bestDist, bestIdx = d, i
		case d > thresholdMeters && inVisit:
			visits = append(visits, MatchedStation{Station: st, SegmentIndex: bestIdx})
			inVisit = false
		}
	}
	if inVisit {
		visits = append(visits, MatchedStation{Station: st, SegmentIndex: bestIdx})
	}
	return visits
}

// mergeCloseVisits merges consecutive visits of the same station when the trace distance
// traveled between them is below minGapMeters, treating them as GPS/threshold jitter around a
// single real pass rather than two distinct crossings.
func mergeCloseVisits(visits []MatchedStation, trace []LatLon, minGapMeters float64) []MatchedStation {
	if len(visits) < 2 {
		return visits
	}

	merged := []MatchedStation{visits[0]}
	for _, v := range visits[1:] {
		last := &merged[len(merged)-1]
		if traceDistanceMeters(trace, last.SegmentIndex, v.SegmentIndex) < minGapMeters {
			continue // within jitter range of the previous visit: drop it
		}
		merged = append(merged, v)
	}
	return merged
}

// traceDistanceMeters sums the haversine length of the trace between segment indices from and to.
func traceDistanceMeters(trace []LatLon, from, to int) float64 {
	var total float64
	for i := from; i < to && i+1 < len(trace); i++ {
		total += haversineMeters(trace[i], trace[i+1])
	}
	return total
}

// boundingBox returns a lat/lon box around trace, expanded by marginMeters.
func boundingBox(trace []LatLon, marginMeters float64) (minLat, maxLat, minLon, maxLon float64) {
	minLat, maxLat = trace[0].Lat, trace[0].Lat
	minLon, maxLon = trace[0].Lon, trace[0].Lon
	for _, p := range trace[1:] {
		if p.Lat < minLat {
			minLat = p.Lat
		}
		if p.Lat > maxLat {
			maxLat = p.Lat
		}
		if p.Lon < minLon {
			minLon = p.Lon
		}
		if p.Lon > maxLon {
			maxLon = p.Lon
		}
	}
	// ~111km per degree of latitude; longitude degrees shrink with cos(lat), but a fixed
	// generous margin in degrees is simpler and safe (only used to cheaply discard far stations).
	marginDeg := marginMeters / 111000.0
	return minLat - marginDeg, maxLat + marginDeg, minLon - marginDeg, maxLon + marginDeg
}

// Segment is a detected toll crossing: either a closed-network entry/exit pair, a single open
// barrier, or an incomplete closed-network entry with no matching exit found on this trace.
type Segment struct {
	Network        string // OpenTollData network_name (closed networks only), empty for open barriers
	Operator       string
	Type           string // "open" or "close"
	Entry          string
	Exit           *string
	EstimatedPrice *money.Cents // class 1 (light vehicle) estimate, nil if unavailable
}

// BuildSegments groups chronologically-ordered matched stations into toll segments and prices
// each one against the dataset (class 1 / light vehicle only).
func (d *Dataset) BuildSegments(matches []MatchedStation) []Segment {
	var segments []Segment

	i := 0
	for i < len(matches) {
		st := matches[i].Station

		if st.Type != "close" {
			seg := Segment{Type: st.Type, Operator: st.Operator, Entry: st.Name}
			d.priceSegment(&seg)
			segments = append(segments, seg)
			i++
			continue
		}

		network := d.NetworkByStation[st.Name]
		seen := map[string]bool{st.Name: true}
		j := i
		for j+1 < len(matches) {
			next := matches[j+1].Station
			if next.Type != "close" || network == "" || d.NetworkByStation[next.Name] != network || seen[next.Name] {
				break // different network/type, or a station already seen in this run (exit + re-entry)
			}
			seen[next.Name] = true
			j++
		}

		seg := Segment{Network: network, Operator: st.Operator, Type: "close", Entry: st.Name}
		if j > i {
			exit := matches[j].Station.Name
			seg.Exit = &exit
		}
		d.priceSegment(&seg)
		segments = append(segments, seg)
		i = j + 1
	}

	return segments
}

// priceSegment fills seg.EstimatedPrice from the dataset's class 1 price tables, if available.
func (d *Dataset) priceSegment(seg *Segment) {
	if seg.Type != "close" {
		if p, ok := d.OpenPrice[seg.Entry]; ok {
			seg.EstimatedPrice = &p
		}
		return
	}
	if seg.Exit == nil {
		return
	}
	if exits, ok := d.ClosedPrice[seg.Entry]; ok {
		if p, ok := exits[*seg.Exit]; ok {
			seg.EstimatedPrice = &p
		}
	}
}
