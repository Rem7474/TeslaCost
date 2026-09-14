package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdatePreTeslaMateEnergyValidation(t *testing.T) {
	h := &VehicleHandler{}

	cases := []struct {
		name       string
		body       SavePreTeslaMateEnergyRequest
		wantStatus int
	}{
		{
			name: "negative consumption",
			body: func() SavePreTeslaMateEnergyRequest {
				c := -10.0
				return SavePreTeslaMateEnergyRequest{PreTeslaMateKwh100km: &c}
			}(),
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "excessive consumption",
			body: func() SavePreTeslaMateEnergyRequest {
				c := 150.0
				return SavePreTeslaMateEnergyRequest{PreTeslaMateKwh100km: &c}
			}(),
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "negative tariff",
			body: func() SavePreTeslaMateEnergyRequest {
				p := -0.22
				return SavePreTeslaMateEnergyRequest{PreTeslaMateEurPerKwh: &p}
			}(),
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "excessive tariff",
			body: func() SavePreTeslaMateEnergyRequest {
				p := 15.0
				return SavePreTeslaMateEnergyRequest{PreTeslaMateEurPerKwh: &p}
			}(),
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.body)
			req := httptest.NewRequest(http.MethodPut, "/api/vehicles/123/pre-teslamate-energy", bytes.NewReader(b))
			w := httptest.NewRecorder()
			h.UpdatePreTeslaMateEnergy(w, req)
			if w.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d (body: %s)", tc.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}
