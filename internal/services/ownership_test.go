package services

import (
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func cents(v int64) *money.Cents { c := money.Cents(v); return &c }
func intPtr(v int) *int          { return &v }

func TestOwnershipPurchaseDepreciationAndSale(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	o := &models.VehicleOwnership{
		AcquisitionType: models.AcquisitionCash, StartDate: start,
		PurchasePrice: cents(4500000), PurchaseFees: cents(100000), Incentives: cents(500000),
		ExpectedResaleValue: cents(2000000), ExpectedHoldingMonths: intPtr(48),
	}
	// Base 41,000 € − resale 20,000 € = 21,000 € over 48 months: half after 24 months.
	c := ComputeOwnershipCosts(o, start.AddDate(2, 0, 0), 0)
	if len(c.Missing) != 0 || c.Depreciation < 1049000 || c.Depreciation > 1051000 {
		t.Fatalf("expected ~10,500 € depreciation, got %s (%v)", c.Depreciation, c.Missing)
	}

	// Sold after 30 months for 25,000 €: realized depreciation 16,000 €, frozen afterwards.
	end := start.AddDate(2, 6, 0)
	o.EndDate, o.SalePrice = &end, cents(2500000)
	if c := ComputeOwnershipCosts(o, start.AddDate(5, 0, 0), 0); c.Depreciation != 1600000 {
		t.Fatalf("expected realized depreciation 16,000 €, got %s", c.Depreciation)
	}
}

func TestOwnershipLoanRequiresLoanTerms(t *testing.T) {
	o := &models.VehicleOwnership{
		AcquisitionType: models.AcquisitionLoan, StartDate: time.Now().AddDate(-1, 0, 0),
		PurchasePrice: cents(4000000), ExpectedResaleValue: cents(2000000), ExpectedHoldingMonths: intPtr(60),
	}
	if c := ComputeOwnershipCosts(o, time.Now(), 0); len(c.Missing) != 1 {
		t.Fatalf("expected the incomplete loan to be reported, got %v", c.Missing)
	}
}

func TestOwnershipLeaseAdjustments(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	allowance, price := 15000.0, 0.10
	o := &models.VehicleOwnership{
		AcquisitionType: models.AcquisitionLOA, StartDate: start,
		LeaseDownPayment: cents(300000), LeaseFees: cents(30000), LeaseMonthlyRent: cents(45000), LeaseDurationMonths: intPtr(36),
		LeaseEndFeesEstimate: cents(90000), LeaseKmAllowancePerYear: &allowance, LeaseExcessKmPrice: &price,
		LeasePurchaseOptionPrice: cents(2200000), LeaseIncludesMaintenance: true,
	}

	// After 12 of 36 months with 20,000 km driven.
	now := start.AddDate(1, 0, 0)
	c := ComputeOwnershipCosts(o, now, 20000)
	if len(c.Missing) != 0 {
		t.Fatalf("unexpected missing settings: %v", c.Missing)
	}
	// Prepaid 3,300 €: one third consumed → adjustment ≈ −2,200 €
	if c.LeasePrepaidAdjustment > -219000 || c.LeasePrepaidAdjustment < -221000 {
		t.Fatalf("expected prepaid adjustment ≈ -2,200 €, got %s", c.LeasePrepaidAdjustment)
	}
	// End fees 900 € accrued one third, not paid yet → ≈ +300 €
	if c.LeaseEndFeesAdjustment < 29000 || c.LeaseEndFeesAdjustment > 31000 {
		t.Fatalf("expected end fees accrual ≈ 300 €, got %s", c.LeaseEndFeesAdjustment)
	}
	// Allowance to date ≈ 15,000 km: ≈ 5,000 km over → ≈ 500 €; projected 60,000 − 45,000 = 15,000 km → ≈ 1,500 €
	if c.LeaseExcessKm < 49000 || c.LeaseExcessKm > 51000 || c.LeaseExcessKmProjected < 149000 || c.LeaseExcessKmProjected > 151000 {
		t.Fatalf("unexpected excess mileage: accrued %s, projected %s", c.LeaseExcessKm, c.LeaseExcessKmProjected)
	}
	if !c.IncludesMaintenance || c.IncludesInsurance {
		t.Fatalf("unexpected included services: %+v", c)
	}

	// Purchase option exercised at term: lease adjustments settle, depreciation starts from the option price.
	exercised := start.AddDate(3, 0, 0)
	resale, months := cents(1200000), intPtr(24)
	o.OptionExercisedDate, o.ExpectedResaleValue, o.ExpectedHoldingMonths = &exercised, resale, months
	c = ComputeOwnershipCosts(o, exercised.AddDate(1, 0, 0), 60000)
	if c.LeasePrepaidAdjustment != 0 || c.LeaseEndFeesAdjustment != 0 || c.LeaseExcessKm != 0 || c.IncludesMaintenance {
		t.Fatalf("lease adjustments must be settled after the option, got %+v", c)
	}
	// 12 calendar months including 29 February 2028 → slightly above 5,000 €
	if c.Depreciation < 498000 || c.Depreciation > 502000 {
		t.Fatalf("expected ~5,000 € depreciation after exercising the option, got %s", c.Depreciation)
	}
}

func TestOwnershipMissingContract(t *testing.T) {
	if c := ComputeOwnershipCosts(nil, time.Now(), 0); len(c.Missing) != 1 {
		t.Fatalf("expected a missing acquisition warning, got %v", c.Missing)
	}
	lld := &models.VehicleOwnership{AcquisitionType: models.AcquisitionLLD, StartDate: time.Now()}
	if c := ComputeOwnershipCosts(lld, time.Now(), 0); len(c.Missing) != 1 {
		t.Fatalf("expected an incomplete lease warning, got %v", c.Missing)
	}
}
