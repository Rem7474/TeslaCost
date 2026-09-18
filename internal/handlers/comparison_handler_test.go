package handlers

import (
	"strings"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func validComparisonRequest() SaveComparisonRequest {
	vid := "veh-1"
	return SaveComparisonRequest{
		VehicleID: &vid,
		Name:      "  Vs SUV essence ",
		Mode:      models.ComparisonModeRetrospective,
		AnnualKm:  12000,
		Years:     5,
		ICE: models.ICEInputs{
			FuelType: "SP95_E10", LPer100Km: 6.5, FuelPrice: 1.75,
			PurchasePrice: money.FromFloat(25000), ResaleValue: money.FromFloat(10000),
			MaintenanceYearly: money.FromFloat(700), InsuranceYearly: money.FromFloat(650),
		},
	}
}

func TestValidateComparisonRequest(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(r *SaveComparisonRequest)
		wantErr string
	}{
		{"valid retrospective", func(r *SaveComparisonRequest) {}, ""},
		{"empty name", func(r *SaveComparisonRequest) { r.Name = "   " }, "Nom invalide"},
		{"long name", func(r *SaveComparisonRequest) { r.Name = strings.Repeat("a", 101) }, "Nom invalide"},
		{"unknown mode", func(r *SaveComparisonRequest) { r.Mode = "OTHER" }, "Mode invalide"},
		{"zero km", func(r *SaveComparisonRequest) { r.AnnualKm = 0 }, "Kilométrage annuel"},
		{"too many km", func(r *SaveComparisonRequest) { r.AnnualKm = 500_000 }, "Kilométrage annuel"},
		{"zero years", func(r *SaveComparisonRequest) { r.Years = 0 }, "Durée invalide"},
		{"too many years", func(r *SaveComparisonRequest) { r.Years = 16 }, "Durée invalide"},
		{"unknown fuel", func(r *SaveComparisonRequest) { r.ICE.FuelType = "KEROSENE" }, "Carburant invalide"},
		{"zero consumption", func(r *SaveComparisonRequest) { r.ICE.LPer100Km = 0 }, "Consommation thermique"},
		{"negative fuel price", func(r *SaveComparisonRequest) { r.ICE.FuelPrice = -1 }, "Prix du carburant"},
		{"negative purchase", func(r *SaveComparisonRequest) { r.ICE.PurchasePrice = -1 }, "positif"},
		{"inflation out of range", func(r *SaveComparisonRequest) { r.Options.FuelInflationPct = 50 }, "Inflation invalide"},
		{"retrospective without vehicle", func(r *SaveComparisonRequest) { r.VehicleID = nil }, "véhicule est requis"},
		{"projection without EV", func(r *SaveComparisonRequest) { r.Mode = models.ComparisonModeProjection }, "véhicule électrique sont requises"},
		{"projection bad EV consumption", func(r *SaveComparisonRequest) {
			r.Mode = models.ComparisonModeProjection
			r.EV = &models.EVInputs{KwhPer100Km: 0, EurPerKwh: 0.2}
		}, "Consommation électrique"},
		{"projection bad electricity price", func(r *SaveComparisonRequest) {
			r.Mode = models.ComparisonModeProjection
			r.EV = &models.EVInputs{KwhPer100Km: 16, EurPerKwh: 9}
		}, "Prix de l'électricité"},
		{"projection negative EV amount", func(r *SaveComparisonRequest) {
			r.Mode = models.ComparisonModeProjection
			r.EV = &models.EVInputs{KwhPer100Km: 16, EurPerKwh: 0.2, PurchasePrice: -5}
		}, "positif"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validComparisonRequest()
			tt.mutate(&req)
			err := validateComparisonRequest(&req)
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

func TestValidateComparisonRequestNormalizesLinks(t *testing.T) {
	req := validComparisonRequest()
	req.EV = &models.EVInputs{KwhPer100Km: 16, EurPerKwh: 0.2}
	if err := validateComparisonRequest(&req); err != nil {
		t.Fatal(err)
	}
	if req.Name != "Vs SUV essence" || req.EV != nil {
		t.Errorf("retrospective must trim the name and drop EV inputs, got %q / %v", req.Name, req.EV)
	}

	proj := validComparisonRequest()
	proj.Mode = models.ComparisonModeProjection
	proj.EV = &models.EVInputs{KwhPer100Km: 16, EurPerKwh: 0.2, PurchasePrice: money.FromFloat(38000)}
	if err := validateComparisonRequest(&proj); err != nil {
		t.Fatal(err)
	}
	if proj.VehicleID != nil {
		t.Errorf("projection must not keep a vehicle, got %v", *proj.VehicleID)
	}
}
