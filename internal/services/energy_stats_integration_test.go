package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// The queries feeding the statistics: months in the reporting timezone, EUR conversion of the cost, exclusion of
// what TeslaMate deleted, and the grid/stored energy reading.
func TestIntegrationEnergyStats(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "energy@example.com")
	svc := NewEnergyStatsService(db.Pool, "Europe/Paris")

	kwh := func(v float64) *float64 { return &v }
	drive := func(tmID int, start time.Time, km float64, energy *float64) *models.Drive {
		d := mustDrive(t, repo, v.ID, tmID, start, 10000+float64(tmID)*200, km)
		d.EnergyConsumedKwh = energy
		if _, err := repo.UpsertTeslaMateDrive(ctx, d); err != nil {
			t.Fatal(err)
		}
		return d
	}

	// 23:30 UTC on 31 January is 00:30 on 1 February in Paris: the drive belongs to February.
	drive(1, time.Date(2026, 1, 31, 23, 30, 0, 0, time.UTC), 100, kwh(15))
	drive(2, time.Date(2026, 2, 10, 9, 0, 0, 0, time.UTC), 200, nil) // no energy reading: distance only
	deleted := drive(3, time.Date(2026, 2, 12, 9, 0, 0, 0, time.UTC), 300, kwh(60))

	charge := func(tmID int, start time.Time, hours float64, added float64, used *float64, cost *money.Cents, currency string, fx *float64) {
		end := start.Add(time.Duration(hours * float64(time.Hour)))
		c := &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &tmID, Date: start, EndDate: &end, KwhAdded: added, KwhUsed: used, Cost: cost, Currency: currency, FxRate: fx}
		if _, err := repo.UpsertTeslaMateCharge(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	eur := func(v float64) *money.Cents { c := money.FromFloat(v); return &c }

	charge(1, time.Date(2026, 2, 3, 18, 0, 0, 0, time.UTC), 4, 30, kwh(33), eur(6), "EUR", nil)  // AC
	charge(2, time.Date(2026, 2, 5, 12, 0, 0, 0, time.UTC), 0.5, 40, nil, eur(20), "EUR", nil)   // DC
	charge(3, time.Date(2026, 2, 6, 12, 0, 0, 0, time.UTC), 5, 10, kwh(12), nil, "EUR", nil)     // no tariff
	charge(4, time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC), 5, 10, kwh(12), eur(10), "GBP", nil) // foreign cost without rate: unknown
	// Only manual entries carry a foreign currency with its rate.
	fx := 1.2
	manualEnd := time.Date(2026, 2, 8, 17, 0, 0, 0, time.UTC)
	manual := &models.ChargeLog{VehicleID: v.ID, Date: time.Date(2026, 2, 8, 12, 0, 0, 0, time.UTC), EndDate: &manualEnd, KwhAdded: 10, Cost: eur(5), Currency: "GBP", FxRate: &fx} // 5 GBP = 6 EUR
	if err := repo.CreateManualCharge(ctx, manual); err != nil {
		t.Fatal(err)
	}

	// A charge deleted in TeslaMate no longer counts.
	charge(6, time.Date(2026, 2, 9, 12, 0, 0, 0, time.UTC), 3, 99, kwh(100), eur(50), "EUR", nil)
	if res, err := repo.ReconcileTeslaMateRecords(ctx, "charges", v.ID, ptrTime(time.Date(2026, 2, 9, 0, 0, 0, 0, time.UTC)), nil); err != nil || res.Marked != 1 {
		t.Fatalf("expected the charge to be flagged deleted, got %+v (err %v)", res, err)
	}
	// ... and neither does a deleted drive.
	if res, err := repo.ReconcileTeslaMateRecords(ctx, "drives", v.ID, ptrTime(deleted.StartTime.Add(-time.Hour)), nil); err != nil || res.Marked != 1 {
		t.Fatalf("expected the drive to be flagged deleted, got %+v (err %v)", res, err)
	}

	got, err := svc.Compute(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Months) != 1 || got.Months[0].Month != "2026-02" {
		t.Fatalf("expected the 23:30 UTC drive in the Paris month of February, got %+v", got.Months)
	}
	m := got.Months[0]
	if m.DistanceKm != 300 {
		t.Errorf("distance: got %v, want 300 (deleted drive excluded)", m.DistanceKm)
	}
	eq(t, "consumption", m.ConsumptionKwh100km, 15.0) // 15 kWh over the 100 km with a reading
	if m.EnergyCost != money.FromFloat(32) {          // 6 + 20 + 6 (GBP converted); no tariff and no rate stay out
		t.Errorf("energy cost: got %v, want 32.00", m.EnergyCost)
	}
	if m.KwhAdded != 100 { // 30 + 40 + 10 + 10 + 10, deleted session excluded
		t.Errorf("kWh added: got %v, want 100", m.KwhAdded)
	}
	if got.Summary.SessionsWithoutCost != 2 {
		t.Errorf("sessions without cost: got %d, want 2 (no tariff, GBP without rate)", got.Summary.SessionsWithoutCost)
	}
	// 50 kWh stored over 57 drawn: neither the DC session nor the manual entry has a grid reading
	eq(t, "efficiency", m.ChargeEfficiency, 0.877)
}

func ptrTime(t time.Time) *time.Time { return &t }
