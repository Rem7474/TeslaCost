package models

import "testing"

func TestDriveIsHighway(t *testing.T) {
	ptrFloat := func(v float64) *float64 { return &v }
	ptrInt := func(v int) *int { return &v }

	tests := []struct {
		name       string
		distanceKm float64
		speedAvg   *float64
		speedMax   *int
		want       bool
	}{
		{
			name:       "nil speeds",
			distanceKm: 50,
			speedAvg:   nil,
			speedMax:   nil,
			want:       false,
		},
		{
			name:       "standard long fast highway drive",
			distanceKm: 42,
			speedAvg:   ptrFloat(72.5),
			speedMax:   ptrInt(115),
			want:       true,
		},
		{
			name:       "highway drive slowed down by traffic jam with vmax > 125",
			distanceKm: 35,
			speedAvg:   ptrFloat(52.0),
			speedMax:   ptrInt(131),
			want:       true,
		},
		{
			name:       "short ramp or urban hop with vmax > 125 under 20km threshold",
			distanceKm: 12,
			speedAvg:   ptrFloat(60.0),
			speedMax:   ptrInt(130),
			want:       false,
		},
		{
			name:       "highway drive exactly at 20km with vmax > 125",
			distanceKm: 20,
			speedAvg:   ptrFloat(65.0),
			speedMax:   ptrInt(126),
			want:       true,
		},
		{
			name:       "vmax exactly 125 does not trigger vmax rule alone",
			distanceKm: 25,
			speedAvg:   ptrFloat(60.0),
			speedMax:   ptrInt(125),
			want:       false,
		},
		{
			name:       "secondary road 50km at 65 km/h",
			distanceKm: 50,
			speedAvg:   ptrFloat(65.0),
			speedMax:   ptrInt(85),
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Drive{
				DistanceKm: tt.distanceKm,
				SpeedAvg:   tt.speedAvg,
				SpeedMax:   tt.speedMax,
			}
			if got := d.IsHighway(); got != tt.want {
				t.Errorf("Drive.IsHighway() = %v, want %v (dist=%.1f, avg=%v, max=%v)",
					got, tt.want, tt.distanceKm, tt.speedAvg, tt.speedMax)
			}
		})
	}
}
