package handlers

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/teslacost/teslacost/internal/models"
)

func decodeOwnership(t *testing.T, body string) *SaveOwnershipRequest {
	t.Helper()
	var req SaveOwnershipRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	return &req
}

func TestBuildOwnershipValidation(t *testing.T) {
	cases := []struct {
		name, body, wantErr string
	}{
		{"unknown type", `{"acquisition_type":"GIFT","start_date":"2025-01-01"}`, "mode d'acquisition invalide"},
		{"missing start", `{"acquisition_type":"CASH","purchase_price":40000}`, "date de début"},
		{"cash without price", `{"acquisition_type":"CASH","start_date":"2025-01-01"}`, "prix d'achat"},
		{"resale above cost", `{"acquisition_type":"CASH","start_date":"2025-01-01","purchase_price":30000,"incentives":5000,"expected_resale_value":26000}`, "revente estimée"},
		{"loan above price", `{"acquisition_type":"LOAN","start_date":"2025-01-01","purchase_price":30000,"loan_amount":35000,"loan_rate_pct":4,"loan_duration_months":60}`, "dépasse le coût d'achat"},
		{"loan without rate", `{"acquisition_type":"LOAN","start_date":"2025-01-01","purchase_price":30000,"loan_amount":20000,"loan_duration_months":60}`, "taux du crédit"},
		{"lease without rent", `{"acquisition_type":"LLD","start_date":"2025-01-01","lease_duration_months":36}`, "loyer mensuel"},
		{"option without price", `{"acquisition_type":"LOA","start_date":"2025-01-01","lease_monthly_rent":400,"lease_duration_months":36,"option_exercised_date":"2028-01-01"}`, "prix de l'option"},
		{"lease resale without option", `{"acquisition_type":"LLD","start_date":"2025-01-01","lease_monthly_rent":400,"lease_duration_months":36,"end_date":"2028-01-01","sale_price":10000}`, "sans levée d'option"},
		{"sale without end date", `{"acquisition_type":"CASH","start_date":"2025-01-01","purchase_price":30000,"sale_price":15000}`, "date de fin"},
		{"end before start", `{"acquisition_type":"CASH","start_date":"2025-01-01","purchase_price":30000,"end_date":"2024-01-01"}`, "précède"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildOwnership("v", decodeOwnership(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestBuildOwnershipKeepsOnlyRelevantFields(t *testing.T) {
	o, err := buildOwnership("v", decodeOwnership(t, `{
		"acquisition_type":"lld","start_date":"2025-01-01","lease_monthly_rent":"419.90","lease_duration_months":48,
		"lease_down_payment":2500,"lease_km_allowance_per_year":15000,"lease_excess_km_price":0.12,
		"lease_includes_maintenance":true,"purchase_price":45000,"loan_amount":20000,"lease_purchase_option_price":18000
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if o.AcquisitionType != models.AcquisitionLLD || *o.LeaseMonthlyRent != 41990 || !o.LeaseIncludesMaintenance {
		t.Fatalf("unexpected lease: %+v", o)
	}
	if o.PurchasePrice != nil || o.LoanAmount != nil || o.LeasePurchaseOptionPrice != nil {
		t.Fatal("purchase, loan and purchase option fields do not apply to an LLD")
	}
}
