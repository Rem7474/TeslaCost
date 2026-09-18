package tolldata

import "math"

const earthRadiusMeters = 6371000.0

// LatLon is a WGS84 coordinate.
type LatLon struct {
	Lat float64
	Lon float64
}

// haversineMeters returns the great-circle distance between two points, in meters.
func haversineMeters(a, b LatLon) float64 {
	lat1, lon1 := degToRad(a.Lat), degToRad(a.Lon)
	lat2, lon2 := degToRad(b.Lat), degToRad(b.Lon)

	dLat := lat2 - lat1
	dLon := lon2 - lon1

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadiusMeters * c
}

// pointToSegmentMeters approximates the shortest distance from p to the segment [a, b], in meters.
// It projects the points onto a local equirectangular plane (centered on the segment), which is
// accurate enough at the scale of a single road segment between two consecutive GPS samples.
func pointToSegmentMeters(p, a, b LatLon) float64 {
	// Local planar projection centered on 'a', x = longitude scaled by cos(latitude), y = latitude.
	centerLat := degToRad(a.Lat)
	cosLat := math.Cos(centerLat)

	toXY := func(pt LatLon) (float64, float64) {
		x := degToRad(pt.Lon-a.Lon) * cosLat * earthRadiusMeters
		y := degToRad(pt.Lat-a.Lat) * earthRadiusMeters
		return x, y
	}

	ax, ay := toXY(a)
	bx, by := toXY(b)
	px, py := toXY(p)

	dx, dy := bx-ax, by-ay
	lenSq := dx*dx + dy*dy
	if lenSq == 0 {
		return haversineMeters(p, a)
	}

	t := ((px-ax)*dx + (py-ay)*dy) / lenSq
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}

	closestX := ax + t*dx
	closestY := ay + t*dy
	ddx, ddy := px-closestX, py-closestY
	return math.Sqrt(ddx*ddx + ddy*ddy)
}

func degToRad(deg float64) float64 {
	return deg * math.Pi / 180
}
