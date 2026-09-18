package tolldata

import "sort"

// DefaultThresholdMeters is the maximum distance between a drive's GPS trace and a toll
// station for that station to be considered crossed.
const DefaultThresholdMeters = 150.0

// MatchedStation is a toll station whose distance to the drive's trace fell under the threshold.
type MatchedStation struct {
	Station      Station
	SegmentIndex int // index into the trace segment [i, i+1] where the minimum distance occurred
}

// DetectCrossings returns the stations crossed by trace, in chronological order.
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

		best := -1
		bestDist := thresholdMeters
		for i := 0; i < len(trace)-1; i++ {
			d := pointToSegmentMeters(LatLon{Lat: st.Lat, Lon: st.Lon}, trace[i], trace[i+1])
			if d <= bestDist {
				bestDist = d
				best = i
			}
		}
		if best >= 0 {
			matches = append(matches, MatchedStation{Station: st, SegmentIndex: best})
		}
	}

	sort.SliceStable(matches, func(i, j int) bool {
		return matches[i].SegmentIndex < matches[j].SegmentIndex
	})
	return matches
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
	Network  string // OpenTollData network_name (closed networks only), empty for open barriers
	Operator string
	Type     string // "open" or "close"
	Entry    string
	Exit     *string
}

// BuildSegments groups chronologically-ordered matched stations into toll segments.
func BuildSegments(matches []MatchedStation, networkByStation map[string]string) []Segment {
	var segments []Segment

	for i := 0; i < len(matches); i++ {
		st := matches[i].Station

		if st.Type != "close" {
			segments = append(segments, Segment{
				Type:     st.Type,
				Operator: st.Operator,
				Entry:    st.Name,
			})
			continue
		}

		network := networkByStation[st.Name]
		if i+1 < len(matches) {
			next := matches[i+1].Station
			if next.Type == "close" && network != "" && networkByStation[next.Name] == network {
				exit := next.Name
				segments = append(segments, Segment{
					Network:  network,
					Operator: st.Operator,
					Type:     "close",
					Entry:    st.Name,
					Exit:     &exit,
				})
				i++ // consume the exit station too
				continue
			}
		}

		// Closed-network entry with no compatible exit found (end of trace, GPS gap, ...).
		segments = append(segments, Segment{
			Network:  network,
			Operator: st.Operator,
			Type:     "close",
			Entry:    st.Name,
		})
	}

	return segments
}
