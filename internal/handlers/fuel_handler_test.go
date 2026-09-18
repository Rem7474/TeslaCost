package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func cents(v float64) *money.Cents { c := money.FromFloat(v); return &c }
func f64(v float64) *float64       { return &v }

func TestBuildFuelLog(t *testing.T) {
	falseV := false
	tests := []struct {
		name    string
		req     SaveFuelLogRequest
		wantErr string
		check   func(t *testing.T, f *models.FuelLog)
	}{
		{
			name: "amount only",
			req:  SaveFuelLogRequest{Date: "2026-03-01", Odometer: 12000, Amount: cents(75)},
			check: func(t *testing.T, f *models.FuelLog) {
				if f.Amount != money.FromFloat(75) || f.Liters != nil || f.PricePerLiter != nil || !f.IsFullTank {
					t.Errorf("unexpected: %+v", f)
				}
			},
		},
		{
			name: "amount and liters derive the price",
			req:  SaveFuelLogRequest{Date: "2026-03-01", Odometer: 12000, Amount: cents(75), Liters: f64(40)},
			check: func(t *testing.T, f *models.FuelLog) {
				if f.PricePerLiter == nil || *f.PricePerLiter != 1.875 {
					t.Errorf("price = %v, want 1.875", f.PricePerLiter)
				}
			},
		},
		{
			name: "liters and price derive the amount",
			req:  SaveFuelLogRequest{Date: "2026-03-01", Odometer: 12000, Liters: f64(40), PricePerLiter: f64(1.8), IsFullTank: &falseV},
			check: func(t *testing.T, f *models.FuelLog) {
				if f.Amount != money.FromFloat(72) || f.IsFullTank {
					t.Errorf("unexpected: %+v", f)
				}
			},
		},
		{
			name: "amount and price derive the liters",
			req:  SaveFuelLogRequest{Date: "2026-03-01", Odometer: 12000, Amount: cents(72), PricePerLiter: f64(1.8)},
			check: func(t *testing.T, f *models.FuelLog) {
				if f.Liters == nil || *f.Liters != 40 {
					t.Errorf("liters = %v, want 40", f.Liters)
				}
			},
		},
		{name: "no amount", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: 12000}, wantErr: "Montant"},
		{name: "liters without price or amount", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: 12000, Liters: f64(40)}, wantErr: "Montant"},
		{name: "bad date", req: SaveFuelLogRequest{Date: "nope", Odometer: 12000, Amount: cents(50)}, wantErr: "Date"},
		{name: "negative odometer", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: -1, Amount: cents(50)}, wantErr: "Kilométrage"},
		{name: "huge odometer", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: 3_000_000, Amount: cents(50)}, wantErr: "Kilométrage"},
		{name: "too many liters", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: 1, Amount: cents(50), Liters: f64(900)}, wantErr: "Quantité"},
		{name: "absurd price", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: 1, Amount: cents(50), PricePerLiter: f64(50)}, wantErr: "Prix au litre"},
		{name: "unknown fuel", req: SaveFuelLogRequest{Date: "2026-03-01", Odometer: 1, Amount: cents(50), FuelType: strPtr("KEROSENE")}, wantErr: "Carburant"},
		{
			name: "known fuel and trimmed notes",
			req:  SaveFuelLogRequest{Date: "2026-03-01", Odometer: 1, Amount: cents(50), FuelType: strPtr("DIESEL"), Notes: strPtr("  Leclerc ")},
			check: func(t *testing.T, f *models.FuelLog) {
				if *f.FuelType != "DIESEL" || *f.Notes != "Leclerc" {
					t.Errorf("unexpected: %+v", f)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := buildFuelLog("veh", &tt.req)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if tt.check != nil {
				tt.check(t, f)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

func TestCheckFuelOdometerOrder(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC) }
	others := []models.FuelLog{
		{ID: "a", Date: day(1), Odometer: 10000},
		{ID: "b", Date: day(10), Odometer: 10800},
	}
	tests := []struct {
		name    string
		id      string
		date    time.Time
		odo     float64
		wantErr string
	}{
		{"between the two", "", day(5), 10400, ""},
		{"same day as a neighbour is not compared", "", day(10), 10500, ""},
		{"after the last with a higher odometer", "", day(20), 11000, ""},
		{"lower than an older fill-up", "", day(5), 9000, "plus ancien"},
		{"higher than a newer fill-up", "", day(5), 11000, "plus récent"},
		{"editing a fill-up ignores itself", "b", day(10), 10500, ""},
		{"editing still checks the others", "b", day(10), 9000, "plus ancien"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkFuelOdometerOrder(others, tt.id, tt.date, tt.odo)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePowertrain(t *testing.T) {
	url := "http://teslamate.local"
	empty := "  "
	tests := []struct {
		name       string
		powertrain string
		url        *string
		wantErr    string
	}{
		{"default", "", nil, ""},
		{"EV with TeslaMate", "EV", &url, ""},
		{"ICE without TeslaMate", "ICE", nil, ""},
		{"ICE with blank URL", "ICE", &empty, ""},
		{"ICE with TeslaMate", "ICE", &url, "TeslaMate"},
		{"unknown", "HYBRID", nil, "motorisation invalide"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePowertrain(tt.powertrain, tt.url)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}
