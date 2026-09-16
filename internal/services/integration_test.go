package services

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/migrations"
)

// Integration tests run against a disposable PostgreSQL database:
//   TEST_DATABASE_URL=postgres://user:pass@localhost:5432/teslacost_test?sslmode=disable go test ./internal/services/
// The public schema of that database is dropped and recreated.

func setupIntegrationDB(t *testing.T, legacyMigrations bool) (*database.DB, *database.Repository) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatal(err)
	}
	if legacyMigrations {
		// Simulates a deployment created before migration versioning: files 1-4 applied, no schema_migrations.
		for _, f := range []string{"000001_init_schema.up.sql", "000002_carpooling.up.sql", "000003_tire_enhancements.up.sql", "000004_telemetry_power_and_insurance.up.sql"} {
			sql, _ := migrations.FS.ReadFile(f)
			if _, err := pool.Exec(ctx, string(sql)); err != nil {
				t.Fatalf("legacy migration %s: %v", f, err)
			}
		}
	}
	db := &database.DB{Pool: pool}
	if err := db.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db, database.NewRepository(pool)
}

func mustVehicle(t *testing.T, repo *database.Repository, email string) *models.Vehicle {
	t.Helper()
	ctx := context.Background()
	u, err := repo.CreateUser(ctx, email, "hash")
	if err != nil {
		t.Fatal(err)
	}
	v := &models.Vehicle{UserID: u.ID, Name: "Model 3", TeslaMateAuthType: models.AuthModeNone, CurrentOdometer: 20000}
	if err := repo.CreateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	return v
}

func mustDrive(t *testing.T, repo *database.Repository, vehicleID string, tmID int, start time.Time, startOdo, km float64) *models.Drive {
	t.Helper()
	end := startOdo + km
	speed := 100.0
	d := &models.Drive{VehicleID: vehicleID, TeslaMateDriveID: &tmID, StartTime: start, EndTime: start.Add(time.Hour),
		StartOdometer: &startOdo, EndOdometer: &end, DistanceKm: km, SpeedAvg: &speed, Tags: []string{}}
	if _, err := repo.UpsertTeslaMateDrive(context.Background(), d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestIntegrationMigrationsOnLegacyDatabase(t *testing.T) {
	db, _ := setupIntegrationDB(t, true)
	var count int
	if err := db.Pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	entries, _ := migrations.FS.ReadDir(".")
	expected := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			expected++
		}
	}
	if count != expected {
		t.Fatalf("expected %d recorded migrations, got %d", expected, count)
	}
	// Idempotent second run.
	if err := db.Migrate(context.Background()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestIntegrationDriveExpenseOwnershipAndGroupAllocation(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	alice := mustVehicle(t, repo, "alice@example.com")
	bob := mustVehicle(t, repo, "bob@example.com")

	base := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	d1 := mustDrive(t, repo, alice.ID, 1, base, 10000, 100)
	d2 := mustDrive(t, repo, alice.ID, 2, base.Add(3*time.Hour), 10100, 300)
	bobDrive := mustDrive(t, repo, bob.ID, 1, base, 5000, 50)

	// Cross-tenant references are rejected.
	foreign := &models.DriveExpense{VehicleID: alice.ID, DriveID: &bobDrive.ID, Type: "TOLL", Amount: 500, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, foreign, nil, ""); !errors.Is(err, database.ErrForeignReference) {
		t.Fatalf("expected ErrForeignReference for a foreign drive, got %v", err)
	}
	if err := repo.CreateTripGroup(ctx, &models.TripGroup{VehicleID: alice.ID, Name: "x"}, []string{d1.ID, bobDrive.ID}); !errors.Is(err, database.ErrForeignReference) {
		t.Fatalf("expected ErrForeignReference for a foreign drive in a group, got %v", err)
	}

	// A 40 € toll over two drives (100 km + 300 km) is allocated 10 / 30.
	exp := &models.DriveExpense{VehicleID: alice.ID, Type: "TOLL", Amount: 4000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, exp, []string{d2.ID, d1.ID}, "Paris → Lyon"); err != nil {
		t.Fatal(err)
	}
	if exp.TripGroupID == nil || exp.DriveID != nil {
		t.Fatal("expected the expense to be attached to a trip group only")
	}
	alloc, err := repo.GetTollExpensesForDrives(ctx, alice.ID, []string{d1.ID, d2.ID})
	if err != nil {
		t.Fatal(err)
	}
	if alloc[d1.ID] != 1000 || alloc[d2.ID] != 3000 {
		t.Fatalf("unexpected allocation: %v", alloc)
	}
	perDrive, err := repo.GetDriveExpensesByDriveID(ctx, alice.ID, d1.ID)
	if err != nil || len(perDrive) != 1 || perDrive[0].AllocatedAmount == nil || *perDrive[0].AllocatedAmount != 1000 {
		t.Fatalf("expected one expense allocated 10 € to d1, got %+v (err %v)", perDrive, err)
	}

	// Editing the expense (same drives) keeps the same group instead of creating a new one.
	groupID := *exp.TripGroupID
	exp.Amount = 4200
	if err := repo.SaveDriveExpense(ctx, exp, []string{d1.ID, d2.ID}, "Paris → Lyon"); err != nil {
		t.Fatal(err)
	}
	if exp.TripGroupID == nil || *exp.TripGroupID != groupID {
		t.Fatal("expected the trip group to be reused on edit")
	}
	groups, _ := repo.ListTripGroups(ctx, alice.ID)
	if len(groups) != 1 {
		t.Fatalf("expected a single trip group, got %d", len(groups))
	}

	// Editing without links keeps nothing hidden: the list exposes group drive IDs for the UI.
	list, _ := repo.ListDriveExpenses(ctx, alice.ID)
	if len(list) != 1 || len(list[0].TripGroupDriveIDs) != 2 {
		t.Fatalf("expected group drive IDs in listing, got %+v", list)
	}

	// Drives with an allocated toll are no longer "unqualified"; bob's fast drive is.
	if n, _ := repo.CountUnqualifiedDrives(ctx, alice.ID); n != 0 {
		t.Fatalf("expected no unqualified drive for alice, got %d", n)
	}
	if n, _ := repo.CountUnqualifiedDrives(ctx, bob.ID); n != 1 {
		t.Fatalf("expected 1 unqualified drive for bob, got %d", n)
	}
	if err := repo.SetDriveTollReviewed(ctx, bobDrive.ID, bob.ID, true); err != nil {
		t.Fatal(err)
	}
	if n, _ := repo.CountUnqualifiedDrives(ctx, bob.ID); n != 0 {
		t.Fatalf("expected reviewed drive to leave the queue, got %d", n)
	}
}

func TestIntegrationTCOCompletenessRecurringAndInsurance(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "carol@example.com")
	tco := NewTCOService(db.Pool, "Europe/Paris")

	now := time.Now().UTC()
	mustDrive(t, repo, v.ID, 1, now.AddDate(0, -2, 0), 10000, 400)
	mustDrive(t, repo, v.ID, 2, now.AddDate(0, -1, 0), 10900, 100) // 500 km odometer gap before this drive

	// Charges: one priced, one without tariff.
	cost := money.Cents(1200)
	c1, c2 := 1, 2
	if _, err := repo.UpsertTeslaMateCharge(ctx, &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &c1, Date: now.AddDate(0, -1, 0), KwhAdded: 40, Cost: &cost, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertTeslaMateCharge(ctx, &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &c2, Date: now.AddDate(0, 0, -3), KwhAdded: 30, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}

	// No insurance premium recorded yet: reported as missing.
	sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.InsuranceSource != InsuranceSourceNone || !sum.Completeness.InsuranceMissing || sum.InsuranceCost != 0 {
		t.Fatalf("expected missing insurance, got %s %s", sum.InsuranceSource, sum.InsuranceCost)
	}
	if sum.Completeness.ChargesWithoutCost != 1 || sum.Completeness.IsComplete {
		t.Fatalf("expected a charge without cost to be reported, got %+v", sum.Completeness)
	}
	if sum.AvgCostPerKwh != 0.3 {
		t.Fatalf("expected €/kWh computed on priced charges only (0.30), got %.3f", sum.AvgCostPerKwh)
	}
	if sum.OdometerDistanceKm != 1000 || sum.TotalDistanceKm != 500 || sum.DistanceBasisKm != 1000 {
		t.Fatalf("expected odometer-based distance basis, got tracked=%.0f odo=%.0f basis=%.0f", sum.TotalDistanceKm, sum.OdometerDistanceKm, sum.DistanceBasisKm)
	}

	// A monthly recurring insurance started 3 months ago counts 4 occurrences and replaces vehicle settings.
	interval := 1
	start := now.AddDate(0, -3, 0).Add(-time.Hour)
	if err := repo.CreateMaintenanceExpense(ctx, &models.MaintenanceExpense{VehicleID: v.ID, Category: "INSURANCE", Amount: 5000, Currency: "EUR",
		Date: start, IsRecurring: true, RecurrenceIntervalMonths: &interval, Description: "Assurance"}); err != nil {
		t.Fatal(err)
	}
	// A CHF parking without rate is excluded and reported.
	if err := repo.SaveDriveExpense(ctx, &models.DriveExpense{VehicleID: v.ID, Type: "PARKING", Amount: 2000, Currency: "CHF", Date: now}, nil, ""); err != nil {
		t.Fatal(err)
	}
	sum, err = tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.InsuranceSource != InsuranceSourceRecordedExpenses || sum.InsuranceCost != 20000 {
		t.Fatalf("expected 4 × 50 € of recorded insurance, got %s %s", sum.InsuranceSource, sum.InsuranceCost)
	}
	if sum.Completeness.UnconvertedExpenses != 1 || sum.TollsCost != 0 {
		t.Fatalf("expected the CHF expense to be excluded and reported, got %+v tolls=%s", sum.Completeness, sum.TollsCost)
	}
	if sum.TotalCost != 21200 {
		t.Fatalf("expected total 12 € energy + 200 € insurance, got %s", sum.TotalCost)
	}
	var monthlyInsurance money.Cents
	for _, m := range sum.MonthlyCosts {
		monthlyInsurance += m.Insurance
	}
	if monthlyInsurance != 20000 {
		t.Fatalf("monthly timeline must match the total insurance, got %s", monthlyInsurance)
	}
}

func TestIntegrationTiresAndManualCharges(t *testing.T) {
	_, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "dave@example.com")
	other := mustVehicle(t, repo, "eve@example.com")

	// Used tire bought with 12,000 km, mounted at 10,000 km.
	odo := 10000.0
	tire := &models.Tire{VehicleID: &v.ID, Brand: "Michelin", Model: "PS4", Dimension: "235/40 R19", Season: models.TireSeasonSummer,
		PurchaseDate: time.Now(), PurchasePrice: 15000, CurrentPosition: models.TirePosFL, InitialDepthMm: 8, MinLegalDepthMm: 1.6,
		MountedOdometer: &odo, AccumulatedDistanceKm: 12000, EstimatedLifespanKm: 40000}
	if err := repo.CreateTire(ctx, tire); err != nil {
		t.Fatal(err)
	}

	// Sessions of another vehicle's tire cannot be touched.
	if err := repo.CreateTireMountSession(ctx, &models.TireMountSession{TireID: tire.ID, VehicleID: other.ID, Position: models.TirePosFL, MountedDate: time.Now(), MountedOdometer: 1}); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for a foreign tire, got %v", err)
	}

	// Rotation with an odometer lower than the mount odometer is rejected.
	var vErr *database.ValidationError
	if err := repo.QuickRotateTires(ctx, v.ID, "FRONT_BACK", 9000, nil); !errors.As(err, &vErr) {
		t.Fatalf("expected a validation error, got %v", err)
	}
	if err := repo.QuickRotateTires(ctx, v.ID, "FRONT_BACK", 15000, nil); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetTireByID(ctx, tire.ID, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.CurrentPosition != models.TirePosRL || got.AccumulatedDistanceKm != 17000 || got.InitialDistanceKm != 12000 {
		t.Fatalf("expected RL with 12,000 + 5,000 km, got %s %.0f (initial %.0f)", got.CurrentPosition, got.AccumulatedDistanceKm, got.InitialDistanceKm)
	}

	// Manual cost correction survives a TeslaMate resync.
	tmID := 7
	if _, err := repo.UpsertTeslaMateCharge(ctx, &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &tmID, Date: time.Now(), KwhAdded: 50, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	charges, _, _ := repo.ListCharges(ctx, v.ID, true, 10, 0)
	if len(charges) != 1 {
		t.Fatalf("expected one charge without cost, got %d", len(charges))
	}
	manualCost := money.Cents(1850)
	fix := &models.ChargeLog{ID: charges[0].ID, VehicleID: v.ID, Cost: &manualCost, Currency: "EUR"}
	if err := repo.UpdateCharge(ctx, fix); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.UpsertTeslaMateCharge(ctx, &models.ChargeLog{VehicleID: v.ID, TeslaMateChargeID: &tmID, Date: time.Now(), KwhAdded: 50, Currency: "EUR"}); err != nil {
		t.Fatal(err)
	}
	charges, _, _ = repo.ListCharges(ctx, v.ID, false, 10, 0)
	if charges[0].Cost == nil || *charges[0].Cost != 1850 || charges[0].CostSource != "MANUAL" || charges[0].KwhAdded != 50 {
		t.Fatalf("expected manual cost to survive resync, got %+v", charges[0])
	}
	if err := repo.DeleteManualCharge(ctx, v.ID, charges[0].ID); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("TeslaMate charges must not be deletable, got %v", err)
	}
}

func TestIntegrationUpstreamDeletions(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "frank@example.com")
	tco := NewTCOService(db.Pool, "UTC")

	base := time.Now().UTC().AddDate(0, -1, 0)
	var drives []*models.Drive
	for i := 1; i <= 12; i++ {
		drives = append(drives, mustDrive(t, repo, v.ID, i, base.Add(time.Duration(i)*time.Hour), 10000+float64(i)*100, 100))
	}
	toll := &models.DriveExpense{VehicleID: v.ID, Type: "TOLL", Amount: 3000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, toll, []string{drives[10].ID, drives[11].ID}, "Voyage"); err != nil {
		t.Fatal(err)
	}

	// Drive 12 deleted in TeslaMate, incremental window covering drives 11 and 12.
	res, err := repo.ReconcileTeslaMateRecords(ctx, "drives", v.ID, &drives[9].StartTime, []int{11})
	if err != nil || res.Marked != 1 {
		t.Fatalf("expected 1 drive flagged, got %+v (err %v)", res, err)
	}
	list, total, _ := repo.ListDrives(ctx, v.ID, database.DriveFilter{}, 50, 0)
	if total != 11 || len(list) != 11 {
		t.Fatalf("expected deleted drive excluded from listing, got %d", total)
	}
	sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.TotalDistanceKm != 1100 || sum.TollsCost != 3000 {
		t.Fatalf("expected 1,100 km and the toll still paid (30 €), got %.0f km / %s €", sum.TotalDistanceKm, sum.TollsCost)
	}
	alloc, _ := repo.GetTollExpensesForDrives(ctx, v.ID, []string{drives[10].ID})
	if alloc[drives[10].ID] != 3000 {
		t.Fatalf("expected the group toll fully allocated to the remaining drive, got %v", alloc)
	}

	// Guard: an empty API answer over the whole history does not wipe it.
	res, err = repo.ReconcileTeslaMateRecords(ctx, "drives", v.ID, nil, nil)
	if err != nil || !res.Skipped || res.Marked != 0 {
		t.Fatalf("expected mass deletion to be skipped, got %+v (err %v)", res, err)
	}

	// The drive comes back in TeslaMate: restored with its links.
	mustDrive(t, repo, v.ID, 12, drives[11].StartTime, 11200, 100)
	if _, total, _ = repo.ListDrives(ctx, v.ID, database.DriveFilter{}, 50, 0); total != 12 {
		t.Fatalf("expected restored drive, got %d drives", total)
	}
	alloc, _ = repo.GetTollExpensesForDrives(ctx, v.ID, []string{drives[10].ID, drives[11].ID})
	if alloc[drives[10].ID] != 1500 || alloc[drives[11].ID] != 1500 {
		t.Fatalf("expected the group toll split again after restoration, got %v", alloc)
	}
}

func TestIntegrationLedgerAcquisitionAndDepreciation(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "grace@example.com")
	tco := NewTCOService(db.Pool, "Europe/Paris")

	// Bought 24 months ago: 45,000 € − 5,000 € bonus, expected resale 20,000 € after 48 months,
	// odometer 0 at purchase, 30,000 km today (TeslaMate only saw the last 1,000 km).
	purchase := time.Now().AddDate(-2, 0, 0)
	price, bonus, resale, months, odo := money.Cents(4500000), money.Cents(500000), money.Cents(2000000), 48, 0.0
	v.CurrentOdometer = 30000
	if err := repo.UpdateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID: v.ID, AcquisitionType: models.AcquisitionCash, StartDate: purchase, StartOdometer: &odo,
		PurchasePrice: &price, Incentives: &bonus, ExpectedResaleValue: &resale, ExpectedHoldingMonths: &months,
	}); err != nil {
		t.Fatal(err)
	}
	mustDrive(t, repo, v.ID, 1, time.Now().AddDate(0, 0, -5), 29000, 1000)

	// Lease-style financing and tires bought for 800 €, 4 tires with 25 % of their life used.
	interval := 1
	if err := repo.CreateMaintenanceExpense(ctx, &models.MaintenanceExpense{VehicleID: v.ID, Category: "FINANCING", Amount: 1000,
		Currency: "EUR", Date: time.Now().AddDate(0, -2, 0).Add(-time.Hour), IsRecurring: true, RecurrenceIntervalMonths: &interval,
		Description: "Intérêts crédit"}); err != nil {
		t.Fatal(err)
	}
	mountOdo := 20000.0
	for i := 0; i < 4; i++ {
		pos := []models.TirePosition{models.TirePosFL, models.TirePosFR, models.TirePosRL, models.TirePosRR}[i]
		if err := repo.CreateTire(ctx, &models.Tire{VehicleID: &v.ID, Brand: "M", Model: "X", Dimension: "235", Season: models.TireSeasonSummer,
			PurchaseDate: time.Now().AddDate(-1, 0, 0), PurchasePrice: 20000, CurrentPosition: pos, InitialDepthMm: 8, MinLegalDepthMm: 1.6,
			MountedOdometer: &mountOdo, EstimatedLifespanKm: 40000}); err != nil {
			t.Fatal(err)
		}
	}

	sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.AcquisitionCost != 4000000 {
		t.Fatalf("expected acquisition net of bonus 40,000 €, got %s", sum.AcquisitionCost)
	}
	// (40,000 − 20,000) × 24/48 ≈ 10,000 € (± a day of rounding)
	if sum.DepreciationCost < 999000 || sum.DepreciationCost > 1001000 {
		t.Fatalf("expected ~10,000 € depreciation, got %s", sum.DepreciationCost)
	}
	if sum.DistanceBasisKm != 30000 {
		t.Fatalf("expected distance since purchase (30,000 km), got %.0f", sum.DistanceBasisKm)
	}
	if sum.FinancingCost != 3000 || sum.TiresCost != 80000 || sum.TiresAmortizedCost != 20000 {
		t.Fatalf("unexpected financing/tires: %s / %s / %s", sum.FinancingCost, sum.TiresCost, sum.TiresAmortizedCost)
	}
	if sum.TotalCost != 83000 {
		t.Fatalf("expected running cash costs 30 € + 800 €, got %s", sum.TotalCost)
	}
	if sum.FullCost != 3000+20000+sum.DepreciationCost {
		t.Fatalf("full cost must add amortized tires and depreciation, got %s", sum.FullCost)
	}

	// The ledger is the single source: its sum (acquisition excluded) equals the cash total.
	var ledgerTotal money.Cents
	if err := db.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(amount_eur), 0) FROM cost_ledger WHERE vehicle_id = $1 AND category <> 'ACQUISITION'`, v.ID).Scan(&ledgerTotal); err != nil {
		t.Fatal(err)
	}
	var monthly money.Cents
	for _, m := range sum.MonthlyCosts {
		monthly += m.Total
	}
	if ledgerTotal != sum.TotalCost || monthly != sum.TotalCost {
		t.Fatalf("ledger %s, monthly %s and total %s must match", ledgerTotal, monthly, sum.TotalCost)
	}
}

func TestIntegrationOdometerContinuity(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "heidi@example.com")
	base := time.Now().UTC().AddDate(0, 0, -10)

	mustDrive(t, repo, v.ID, 1, base, 10000, 50)                  // 10000 → 10050
	mustDrive(t, repo, v.ID, 2, base.Add(2*time.Hour), 10050, 30) // continuous
	mustDrive(t, repo, v.ID, 3, base.Add(4*time.Hour), 10200, 20) // gap of 120 km
	mustDrive(t, repo, v.ID, 4, base.Add(6*time.Hour), 10150, 10) // regression of 70 km
	d5 := mustDrive(t, repo, v.ID, 5, base.Add(8*time.Hour), 10160, 40)
	if _, err := db.Pool.Exec(ctx, `UPDATE drives SET distance_km = 60 WHERE id = $1`, d5.ID); err != nil { // mismatch: 60 vs 40
		t.Fatal(err)
	}

	gaps, gapKm, anomalies, err := repo.DataQualitySummary(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gaps != 1 || gapKm != 120 || anomalies != 2 {
		t.Fatalf("expected 1 gap of 120 km and 2 anomalies, got %d / %.0f / %d", gaps, gapKm, anomalies)
	}
	issues, err := repo.ListDataQualityIssues(ctx, v.ID, 10)
	if err != nil || len(issues) != 3 {
		t.Fatalf("expected 3 issues, got %+v (err %v)", issues, err)
	}
	if issues[0].Type != database.IssueDistanceMismatch || issues[1].Type != database.IssueOdometerRegression || issues[2].Type != database.IssueOdometerGap {
		t.Fatalf("unexpected issue order/types: %+v", issues)
	}

	sum, err := NewTCOService(db.Pool, "UTC").ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.Completeness.OdometerGaps != 1 || sum.Completeness.OdometerAnomalies != 2 {
		t.Fatalf("expected continuity issues in completeness, got %+v", sum.Completeness)
	}
}

func TestIntegrationOwnershipLoanLeaseAndInsuranceShare(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	tco := NewTCOService(db.Pool, "UTC")
	rates := NewCarpoolService(db.Pool, repo)
	now := time.Now().UTC()

	// Loan: 30,000 € at 4.8 % over 60 months (payment 563.39 €), started 3 months ago:
	// interest 120.00 € then 118.23 € then 116.45 €.
	loanCar := mustVehicle(t, repo, "loan@example.com")
	price, amount, rate, duration, fees := money.Cents(4500000), money.Cents(3000000), 4.8, 60, money.Cents(15000)
	resale, holding := money.Cents(2000000), 60
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID: loanCar.ID, AcquisitionType: models.AcquisitionLoan, StartDate: now.AddDate(0, -3, 0).Add(-time.Hour),
		PurchasePrice: &price, ExpectedResaleValue: &resale, ExpectedHoldingMonths: &holding,
		LoanAmount: &amount, LoanRatePct: &rate, LoanDurationMonths: &duration, LoanFees: &fees,
	}); err != nil {
		t.Fatal(err)
	}
	sum, err := tco.ComputeVehicleTCO(ctx, loanCar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.AcquisitionType != models.AcquisitionLoan || sum.FinancingCost != 15000+12000+11823+11645 {
		t.Fatalf("expected loan fees and 3 interest payments, got %s", sum.FinancingCost)
	}
	if len(sum.MonthlyCosts) > 0 {
		if sum.MonthlyCosts[0].FinancingAmortized != sum.MonthlyCosts[0].Financing-15000+250 {
			t.Fatalf("expected loan fees smoothed in first month, got financing %s amortized %s", sum.MonthlyCosts[0].Financing, sum.MonthlyCosts[0].FinancingAmortized)
		}
	}

	// LOA: 3,000 € down payment, 450 €/month over 36 months, started 12 months ago, 15,000 km/year allowance,
	// maintenance and insurance included, 20,000 km driven.
	leaseCar := mustVehicle(t, repo, "lease@example.com")
	start := now.AddDate(-1, 0, 0).Add(-time.Hour)
	down, rent, leaseMonths, option := money.Cents(300000), money.Cents(45000), 36, money.Cents(2200000)
	allowance, excessPrice, startOdo := 15000.0, 0.10, 0.0
	leaseCar.CurrentOdometer = 20000
	if err := repo.UpdateVehicle(ctx, leaseCar); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID: leaseCar.ID, AcquisitionType: models.AcquisitionLOA, StartDate: start, StartOdometer: &startOdo,
		LeaseDownPayment: &down, LeaseMonthlyRent: &rent, LeaseDurationMonths: &leaseMonths,
		LeaseKmAllowancePerYear: &allowance, LeaseExcessKmPrice: &excessPrice, LeasePurchaseOptionPrice: &option,
		LeaseIncludesMaintenance: true, LeaseIncludesInsurance: true,
	}); err != nil {
		t.Fatal(err)
	}
	mustDrive(t, repo, leaseCar.ID, 1, now.AddDate(0, -1, 0), 19000, 1000)
	sum, err = tco.ComputeVehicleTCO(ctx, leaseCar.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Cash: down payment + 13 rents (months 0..12)
	if sum.FinancingCost != 300000+13*45000 {
		t.Fatalf("expected down payment and 13 rents, got %s", sum.FinancingCost)
	}
	// Verify monthly costs financing smoothing:
	// Month 0 has cash financing = 300,000 + 45,000 = 345,000 cents (3,450 €)
	// But amortized financing = 45,000 + 300,000/36 = 53,333 cents (533.33 €)
	if len(sum.MonthlyCosts) == 0 {
		t.Fatal("expected monthly costs for lease vehicle")
	}
	firstMonth := sum.MonthlyCosts[0]
	if firstMonth.Financing != 345000 {
		t.Fatalf("expected first month cash financing 345,000 cents, got %s", firstMonth.Financing)
	}
	if firstMonth.FinancingAmortized != 53333 {
		t.Fatalf("expected first month amortized financing 53,333 cents, got %s", firstMonth.FinancingAmortized)
	}
	if len(sum.MonthlyCosts) > 1 {
		secondMonth := sum.MonthlyCosts[1]
		if secondMonth.Financing != 45000 {
			t.Fatalf("expected second month cash financing 45,000 cents, got %s", secondMonth.Financing)
		}
		if secondMonth.FinancingAmortized != 53333 {
			t.Fatalf("expected second month amortized financing 53,333 cents, got %s", secondMonth.FinancingAmortized)
		}
	}
	// Economic: down payment spread (≈ 1/3 consumed) + excess mileage ≈ 5,000 km × 0.10 €
	if sum.FinancingFullCost >= sum.FinancingCost || sum.LeaseExcessKmCost < 40000 || sum.LeaseExcessKmProjected == 0 {
		t.Fatalf("unexpected lease economics: full %s, excess %s, projected %s", sum.FinancingFullCost, sum.LeaseExcessKmCost, sum.LeaseExcessKmProjected)
	}
	if sum.InsuranceSource != InsuranceSourceIncluded || sum.Completeness.InsuranceMissing || sum.Completeness.AcquisitionMissing {
		t.Fatalf("insurance included in the lease must not be reported missing: %+v", sum.Completeness)
	}
	r, err := rates.GetVehicleUnitRates(ctx, leaseCar.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.MaintenanceSource != RateSourceIncluded || r.MaintenancePerKm != 0 || r.InsuranceSource != InsuranceSourceIncluded {
		t.Fatalf("included services must not be charged per km: %+v", r)
	}

	// Insurance share: monthly premium of 60 € started 6 months ago (7 premiums), 3,500 km driven since.
	insured := mustVehicle(t, repo, "insured@example.com")
	interval := 1
	if err := repo.CreateMaintenanceExpense(ctx, &models.MaintenanceExpense{VehicleID: insured.ID, Category: "INSURANCE", Amount: 6000,
		Currency: "EUR", Date: now.AddDate(0, -6, 0).Add(-time.Hour), IsRecurring: true, RecurrenceIntervalMonths: &interval,
		Description: "Prime mensuelle"}); err != nil {
		t.Fatal(err)
	}
	mustDrive(t, repo, insured.ID, 1, now.AddDate(0, -3, 0), 1000, 3500)
	r, err = rates.GetVehicleUnitRates(ctx, insured.ID)
	if err != nil {
		t.Fatal(err)
	}
	if r.InsuranceSource != InsuranceSourceRecordedExpenses || r.InsuranceWindowCost == nil || *r.InsuranceWindowCost != 42000 || r.InsurancePerKm != 0.12 {
		t.Fatalf("expected 420 € over 3,500 km = 0.12 €/km, got %+v", r)
	}

	// Sale: recurring premiums stop at the end of ownership.
	end := now.AddDate(0, -2, 0)
	sale := money.Cents(3000000)
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID: insured.ID, AcquisitionType: models.AcquisitionCash, StartDate: now.AddDate(-1, 0, 0),
		PurchasePrice: &price, EndDate: &end, SalePrice: &sale,
	}); err != nil {
		t.Fatal(err)
	}
	sum, err = tco.ComputeVehicleTCO(ctx, insured.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.InsuranceCost != 5*6000 || sum.DepreciationCost != 1500000 {
		t.Fatalf("expected 5 premiums until the sale and realized depreciation 15,000 €, got %s / %s", sum.InsuranceCost, sum.DepreciationCost)
	}
}

func TestIntegrationEditCapabilities(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "edit@example.com")
	other := mustVehicle(t, repo, "intruder@example.com")
	base := time.Now().UTC().AddDate(0, -1, 0)

	// Set of 4 tires created mounted with rough values, then fixed in one batch.
	odo := 5000.0
	var ids []string
	for _, pos := range []models.TirePosition{models.TirePosFL, models.TirePosFR, models.TirePosRL, models.TirePosRR} {
		tire := &models.Tire{VehicleID: &v.ID, Brand: "X", Model: "Y", Dimension: "235", Season: models.TireSeasonSummer,
			PurchaseDate: time.Now(), PurchasePrice: 1, CurrentPosition: pos, InitialDepthMm: 8, MinLegalDepthMm: 1.6,
			MountedOdometer: &odo, EstimatedLifespanKm: 40000}
		if err := repo.CreateTire(ctx, tire); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, tire.ID)
	}
	brand, total, mounted, mountedOdo := "Michelin", money.Cents(99999), base, 1200.0
	if err := repo.BatchUpdateTires(ctx, v.ID, ids, database.TirePatch{Brand: &brand, TotalPrice: &total, MountedDate: &mounted, MountedOdometer: &mountedOdo}); err != nil {
		t.Fatal(err)
	}
	var sum money.Cents
	for _, id := range ids {
		tire, _ := repo.GetTireByID(ctx, id, v.ID)
		sum += tire.PurchasePrice
		if tire.Brand != "Michelin" || tire.MountedOdometer == nil || *tire.MountedOdometer != 1200 {
			t.Fatalf("batch update not applied: %+v", tire)
		}
	}
	if sum != 99999 {
		t.Fatalf("total price must be split exactly, got %s", sum)
	}
	if err := repo.BatchUpdateTires(ctx, other.ID, ids, database.TirePatch{Brand: &brand}); !errors.Is(err, database.ErrForeignReference) {
		t.Fatalf("batch update of foreign tires must be rejected, got %v", err)
	}

	// Dispose a mounted tire: session closed, DISPOSED, amortized cost fully counted.
	at, disposeOdo := time.Now().UTC(), 1100.0
	var vErr *database.ValidationError
	if err := repo.DisposeTire(ctx, v.ID, ids[0], at, &disposeOdo); !errors.As(err, &vErr) {
		t.Fatalf("odometer below the mount odometer must be rejected, got %v", err)
	}
	disposeOdo = 21200
	if err := repo.DisposeTire(ctx, v.ID, ids[0], at, &disposeOdo); err != nil {
		t.Fatal(err)
	}
	disposed, _ := repo.GetTireByID(ctx, ids[0], v.ID)
	if disposed.CurrentPosition != models.TirePosDisposed || disposed.AccumulatedDistanceKm != 20000 || disposed.MountedOdometer != nil {
		t.Fatalf("unexpected disposed tire: %+v", disposed)
	}

	// Wear logs can be corrected and deleted, not across vehicles.
	log := &models.TireLog{TireID: ids[1], Date: base, Odometer: 3000, DepthMm: 7.5}
	if err := repo.AddTireLog(ctx, log); err != nil {
		t.Fatal(err)
	}
	log.DepthMm = 6.9
	if err := repo.UpdateTireLog(ctx, other.ID, log); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("foreign log update must be rejected, got %v", err)
	}
	if err := repo.UpdateTireLog(ctx, v.ID, log); err != nil {
		t.Fatal(err)
	}
	if logs, _ := repo.ListTireLogs(ctx, ids[1]); len(logs) != 1 || logs[0].DepthMm != 6.9 {
		t.Fatalf("log not updated: %+v", logs)
	}
	if err := repo.DeleteTireLog(ctx, v.ID, ids[1], log.ID); err != nil {
		t.Fatal(err)
	}

	// Delete a tire entered by mistake.
	if err := repo.DeleteTire(ctx, other.ID, ids[3]); !errors.Is(err, database.ErrNotFound) {
		t.Fatalf("foreign tire delete must be rejected, got %v", err)
	}
	if err := repo.DeleteTire(ctx, v.ID, ids[3]); err != nil {
		t.Fatal(err)
	}

	// Trip group: rename, change drives, delete while keeping or deleting its expenses.
	d1 := mustDrive(t, repo, v.ID, 1, base, 10000, 100)
	d2 := mustDrive(t, repo, v.ID, 2, base.Add(2*time.Hour), 10100, 100)
	d3 := mustDrive(t, repo, v.ID, 3, base.Add(4*time.Hour), 10200, 200)
	exp := &models.DriveExpense{VehicleID: v.ID, Type: "TOLL", Amount: 3000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, exp, []string{d1.ID, d2.ID}, "Aller"); err != nil {
		t.Fatal(err)
	}
	tg := &models.TripGroup{ID: *exp.TripGroupID, VehicleID: v.ID, Name: "Vacances - aller"}
	if err := repo.UpdateTripGroup(ctx, tg, []string{d1.ID, d2.ID, d3.ID}); err != nil {
		t.Fatal(err)
	}
	groups, _ := repo.ListTripGroups(ctx, v.ID)
	if len(groups) != 1 || groups[0].Name != "Vacances - aller" || len(groups[0].DriveIDs) != 3 || groups[0].DistanceKm != 400 || groups[0].ExpensesTotal != 3000 {
		t.Fatalf("unexpected trip group summary: %+v", groups)
	}
	if err := repo.UpdateTripGroup(ctx, tg, []string{}); !errors.As(err, &vErr) {
		t.Fatalf("an empty trip group must be rejected, got %v", err)
	}
	if err := repo.DeleteTripGroup(ctx, v.ID, tg.ID, false); err != nil {
		t.Fatal(err)
	}
	expenses, _ := repo.ListDriveExpenses(ctx, v.ID)
	if len(expenses) != 1 || expenses[0].TripGroupID != nil {
		t.Fatalf("the expense must be kept unlinked, got %+v", expenses)
	}
	sumTCO, err := NewTCOService(db.Pool, "UTC").ComputeVehicleTCO(ctx, v.ID)
	if err != nil || sumTCO.TollsCost != 3000 {
		t.Fatalf("the unlinked toll must still count in the TCO, got %v (err %v)", sumTCO.TollsCost, err)
	}
}

func TestIntegrationCarpoolLegsAndStops(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "carpool@example.com")
	other := mustVehicle(t, repo, "other-carpool@example.com")
	svc := NewCarpoolService(db.Pool, repo)
	base := time.Now().UTC().AddDate(0, 0, -2)

	// Annecy → Chambéry (50 km) → Grenoble (60 km) → Valence (90 km); a 40 € toll over the whole trip group
	addr := func(s string) *string { return &s }
	var drives []*models.Drive
	for i, leg := range []struct {
		from, to string
		km       float64
	}{{"Annecy, France", "Chambéry, France", 50}, {"Chambéry, France", "Grenoble, France", 60}, {"Grenoble, France", "Valence, France", 90}} {
		d := mustDrive(t, repo, v.ID, i+1, base.Add(time.Duration(i)*2*time.Hour), 10000+float64(i)*100, leg.km)
		if _, err := db.Pool.Exec(ctx, `UPDATE drives SET start_address = $1, end_address = $2 WHERE id = $3`, leg.from, leg.to, d.ID); err != nil {
			t.Fatal(err)
		}
		d.StartAddress, d.EndAddress = addr(leg.from), addr(leg.to)
		drives = append(drives, d)
	}
	toll := &models.DriveExpense{VehicleID: v.ID, Type: "TOLL", Amount: 4000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, toll, []string{drives[0].ID, drives[1].ID, drives[2].ID}, "Alpes"); err != nil {
		t.Fatal(err)
	}

	est, err := svc.EstimateCosts(ctx, v.ID, nil, nil, []string{drives[2].ID, drives[0].ID, drives[1].ID}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(est.Legs) != 3 || *est.Legs[0].StartLabel != "Annecy" || *est.Legs[2].EndLabel != "Valence" {
		t.Fatalf("expected 3 chronological legs with place labels, got %+v", est.Legs)
	}
	if est.StartDate == nil || !est.StartDate.Truncate(time.Microsecond).Equal(base.Truncate(time.Microsecond)) {
		t.Fatalf("expected est.StartDate %v, got %v", base, est.StartDate)
	}
	if est.Legs[0].TollsCost != 1000 || est.Legs[1].TollsCost != 1200 || est.Legs[2].TollsCost != 1800 || est.TollsCost != 4000 {
		t.Fatalf("expected the group toll split 10/12/18 € across legs, got %d %d %d", est.Legs[0].TollsCost, est.Legs[1].TollsCost, est.Legs[2].TollsCost)
	}

	// Anna rides Annecy → Valence, Bruno boards at Grenoble (stop 2)
	trip := &models.CarpoolTrip{VehicleID: v.ID, Title: "Annecy → Valence", Date: base}
	passengers := []models.CarpoolPassenger{
		{PassengerName: "Anna", Seats: 1, AmountPaid: 2500, BoardStopIndex: 0, AlightStopIndex: 3},
		{PassengerName: "Bruno", Seats: 1, AmountPaid: 1000, BoardStopIndex: 2, AlightStopIndex: 3},
	}
	if err := repo.CreateCarpoolTrip(ctx, trip, est.Legs, passengers); err != nil {
		t.Fatal(err)
	}
	if trip.TotalCost != est.TotalCost || trip.DistanceKm != 200 || trip.TotalRevenue != 3500 {
		t.Fatalf("trip totals must be the sum of its legs: %+v", trip)
	}

	trips, err := repo.ListCarpoolTrips(ctx, v.ID)
	if err != nil || len(trips) != 1 || len(trips[0].Legs) != 3 || len(trips[0].Passengers) != 2 {
		t.Fatalf("expected the trip with 3 legs and 2 passengers, got %+v (err %v)", trips, err)
	}
	loaded := trips[0]
	driver, shared := AllocateCarpoolCosts(loaded.Legs, loaded.Passengers)
	if driver+shared != loaded.TotalCost {
		t.Fatalf("shares must add up to the trip cost")
	}
	// Bruno only pays a third of the last leg; Anna half of the first two legs and a third of the last one
	last := loaded.Legs[2].Total()
	if loaded.Passengers[1].CostShare != money.Split(last, 3)[0] {
		t.Fatalf("Bruno's share must be a third of the last leg (%s), got %s", last, loaded.Passengers[1].CostShare)
	}
	if loaded.Legs[0].PassengerSeats != 1 || loaded.Legs[2].PassengerSeats != 2 {
		t.Fatalf("unexpected occupancy: %d / %d", loaded.Legs[0].PassengerSeats, loaded.Legs[2].PassengerSeats)
	}

	// Recalculate carpool trips: add an extra toll and recalculate
	extraToll := &models.DriveExpense{VehicleID: v.ID, Type: "TOLL", Amount: 2000, Currency: "EUR", Date: base}
	if err := repo.SaveDriveExpense(ctx, extraToll, []string{drives[0].ID}, "Tunnel"); err != nil {
		t.Fatal(err)
	}
	recalculated, err := svc.RecalculateTrip(ctx, v.ID, loaded.ID)
	if err != nil {
		t.Fatalf("failed to recalculate trip: %v", err)
	}
	// Initial total toll was 4000, now 4000 + 2000 = 6000
	if recalculated.TollsCost != 6000 {
		t.Fatalf("expected recalculated tolls cost to be 6000, got %d", recalculated.TollsCost)
	}
	if recalculated.TotalRevenue != 3500 {
		t.Fatalf("passenger revenue must remain untouched, got %d", recalculated.TotalRevenue)
	}

	// Invalid stops and foreign drives are rejected
	var vErr *database.ValidationError
	bad := []models.CarpoolPassenger{{PassengerName: "Zoé", Seats: 1, BoardStopIndex: 2, AlightStopIndex: 4}}
	if err := repo.UpdateCarpoolTrip(ctx, trip, est.Legs, bad); !errors.As(err, &vErr) {
		t.Fatalf("expected invalid stops to be rejected, got %v", err)
	}
	foreign := &models.CarpoolTrip{VehicleID: other.ID, Title: "x", Date: base}
	if err := repo.CreateCarpoolTrip(ctx, foreign, est.Legs, nil); !errors.Is(err, database.ErrForeignReference) {
		t.Fatalf("expected foreign drives to be rejected, got %v", err)
	}
}

func TestCarpoolElectricityRecentCharges5Days(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "elec_test@example.com")
	svc := NewCarpoolService(db.Pool, repo)

	now := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	cost1 := money.Cents(1000) // 10 EUR
	cost2 := money.Cents(2000) // 20 EUR

	// Charge 1: 1 day before 'now', 50 kWh, 10 EUR (0.20 €/kWh)
	c1 := &models.ChargeLog{
		VehicleID: v.ID,
		Date:      now.Add(-24 * time.Hour),
		KwhAdded:  50,
		Cost:      &cost1,
		Currency:  "EUR",
	}
	if err := repo.CreateManualCharge(ctx, c1); err != nil {
		t.Fatal(err)
	}

	// Charge 2: 3 days before Charge 1 (within 5-day window), 50 kWh, 20 EUR (0.40 €/kWh)
	c2 := &models.ChargeLog{
		VehicleID: v.ID,
		Date:      c1.Date.Add(-3 * 24 * time.Hour),
		KwhAdded:  50,
		Cost:      &cost2,
		Currency:  "EUR",
	}
	if err := repo.CreateManualCharge(ctx, c2); err != nil {
		t.Fatal(err)
	}

	// 1. Both charges are within 5 days of each other: weighted average (10 + 20) / (50 + 50) = 0.30 €/kWh
	rates, err := svc.GetVehicleUnitRatesAt(ctx, v.ID, &now)
	if err != nil {
		t.Fatal(err)
	}
	if rates.ElectricitySource != RateSourceRecentCharges {
		t.Fatalf("expected RECENT_CHARGES, got %s", rates.ElectricitySource)
	}
	if rates.ElectricityPerKwh != 0.30 {
		t.Fatalf("expected 0.30 €/kWh weighted average, got %f", rates.ElectricityPerKwh)
	}

	// 2. Now move Charge 2 to 7 days before Charge 1 (> 5 days apart)
	if _, err := db.Pool.Exec(ctx, `UPDATE charge_logs SET date = $1 WHERE id = $2`, c1.Date.Add(-7*24*time.Hour), c2.ID); err != nil {
		t.Fatal(err)
	}
	rates2, err := svc.GetVehicleUnitRatesAt(ctx, v.ID, &now)
	if err != nil {
		t.Fatal(err)
	}
	if rates2.ElectricitySource != RateSourceRecentCharges {
		t.Fatalf("expected RECENT_CHARGES, got %s", rates2.ElectricitySource)
	}
	// Only Charge 1 should be used: 10 EUR / 50 kWh = 0.20 €/kWh
	if rates2.ElectricityPerKwh != 0.20 {
		t.Fatalf("expected 0.20 €/kWh (only latest charge), got %f", rates2.ElectricityPerKwh)
	}
}

func TestCarpoolDailyInsuranceAllocation(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "ins_test@example.com")
	svc := NewCarpoolService(db.Pool, repo)

	// 1. Annual insurance of 365.25 EUR (36525 cents) -> 1.00 EUR/day (100 cents/day)
	day1 := time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	interval := 12
	if err := repo.CreateMaintenanceExpense(ctx, &models.MaintenanceExpense{
		VehicleID:                v.ID,
		Category:                 "INSURANCE",
		Amount:                   36525,
		Currency:                 "EUR",
		Date:                     day1.AddDate(0, -1, 0),
		IsRecurring:              true,
		RecurrenceIntervalMonths: &interval,
		Description:              "Assurance annuelle",
	}); err != nil {
		t.Fatal(err)
	}

	// Day 1 drives:
	// d1: carpool leg 1 (40 km)
	// d2: carpool leg 2 (60 km)
	// d3: personal commute (100 km)
	// Total on Day 1 = 200 km
	d1 := mustDrive(t, repo, v.ID, 201, day1, 10000, 40)
	d2 := mustDrive(t, repo, v.ID, 202, day1.Add(4*time.Hour), 10040, 60)
	mustDrive(t, repo, v.ID, 203, day1.Add(8*time.Hour), 10100, 100)

	// Day 2 drive:
	// d4: 50 km (only drive on Day 2 -> total Day 2 = 50 km)
	d4 := mustDrive(t, repo, v.ID, 204, day2, 10200, 50)

	// 2. Estimate Day 1 carpool with [d1, d2]
	est1, err := svc.EstimateCosts(ctx, v.ID, nil, nil, []string{d1.ID, d2.ID}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if est1.DailyInsuranceCost == nil || *est1.DailyInsuranceCost != 100 {
		t.Fatalf("expected DailyInsuranceCost to be 100 cents (1 EUR), got %v", est1.DailyInsuranceCost)
	}
	if len(est1.Legs) != 2 {
		t.Fatalf("expected 2 legs, got %d", len(est1.Legs))
	}
	// Leg 1: 40 km / 200 km * 1.00 EUR = 0.20 EUR = 20 cents
	if est1.Legs[0].InsuranceCost != 20 {
		t.Fatalf("expected Leg 1 insurance 20 cents, got %d", est1.Legs[0].InsuranceCost)
	}
	// Leg 2: 60 km / 200 km * 1.00 EUR = 0.30 EUR = 30 cents
	if est1.Legs[1].InsuranceCost != 30 {
		t.Fatalf("expected Leg 2 insurance 30 cents, got %d", est1.Legs[1].InsuranceCost)
	}
	// Total insurance for trip = 50 cents (50% of the day's insurance)
	if est1.InsuranceCost != 50 {
		t.Fatalf("expected Total trip insurance 50 cents, got %d", est1.InsuranceCost)
	}
	// Effective insurance rate: 50 cents / 100 km = 0.005 EUR/km
	if est1.InsuranceRatePerKm != 0.005 {
		t.Fatalf("expected InsuranceRatePerKm 0.005, got %f", est1.InsuranceRatePerKm)
	}

	// 3. Estimate Day 2 carpool with [d4] (only drive of the day)
	est2, err := svc.EstimateCosts(ctx, v.ID, nil, nil, []string{d4.ID}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Leg 1: 50 km / 50 km * 1.00 EUR = 1.00 EUR = 100 cents (100% of the day)
	if est2.Legs[0].InsuranceCost != 100 || est2.InsuranceCost != 100 {
		t.Fatalf("expected Day 2 insurance 100 cents, got %d (leg: %d)", est2.InsuranceCost, est2.Legs[0].InsuranceCost)
	}

	// 4. Multi-day carpool trip [d1 (day 1), d4 (day 2)]
	estMulti, err := svc.EstimateCosts(ctx, v.ID, nil, nil, []string{d1.ID, d4.ID}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if estMulti.Legs[0].InsuranceCost != 20 {
		t.Fatalf("expected multi-day leg 1 insurance 20 cents, got %d", estMulti.Legs[0].InsuranceCost)
	}
	if estMulti.Legs[1].InsuranceCost != 100 {
		t.Fatalf("expected multi-day leg 2 insurance 100 cents, got %d", estMulti.Legs[1].InsuranceCost)
	}
	if estMulti.InsuranceCost != 120 {
		t.Fatalf("expected multi-day total insurance 120 cents, got %d", estMulti.InsuranceCost)
	}

	// 5. Create trip and verify RecalculateTrip after driving more personal km on Day 1
	trip := &models.CarpoolTrip{
		VehicleID: v.ID,
		Title:     "Covoit Day 1",
		Date:      day1,
	}
	if err := repo.CreateCarpoolTrip(ctx, trip, est1.Legs, nil); err != nil {
		t.Fatal(err)
	}

	// Personal drive of 200 km added on Day 1: total Day 1 is now 400 km
	mustDrive(t, repo, v.ID, 205, day1.Add(10*time.Hour), 10300, 200)

	recalculated, err := svc.RecalculateTrip(ctx, v.ID, trip.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Leg 1: 40 km / 400 km * 1.00 EUR = 0.10 EUR = 10 cents
	if recalculated.Legs[0].InsuranceCost != 10 {
		t.Fatalf("expected recalculated leg 1 insurance 10 cents, got %d", recalculated.Legs[0].InsuranceCost)
	}
	// Leg 2: 60 km / 400 km * 1.00 EUR = 0.15 EUR = 15 cents
	if recalculated.Legs[1].InsuranceCost != 15 {
		t.Fatalf("expected recalculated leg 2 insurance 15 cents, got %d", recalculated.Legs[1].InsuranceCost)
	}
	if recalculated.InsuranceCost != 25 {
		t.Fatalf("expected recalculated total insurance 25 cents, got %d", recalculated.InsuranceCost)
	}

	// 6. Lease with insurance included
	leaseV := mustVehicle(t, repo, "lease_ins@example.com")
	leaseStart := day1.AddDate(-1, 0, 0)
	price := money.Cents(4000000)
	leaseDuration := 36
	if err := repo.SaveVehicleOwnership(ctx, &models.VehicleOwnership{
		VehicleID:              leaseV.ID,
		AcquisitionType:        models.AcquisitionLLD,
		StartDate:              leaseStart,
		PurchasePrice:          &price,
		LeaseDurationMonths:    &leaseDuration,
		LeaseIncludesInsurance: true,
	}); err != nil {
		t.Fatal(err)
	}
	leaseDrive := mustDrive(t, repo, leaseV.ID, 301, day1, 1000, 100)
	leaseEst, err := svc.EstimateCosts(ctx, leaseV.ID, nil, nil, []string{leaseDrive.ID}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if leaseEst.InsuranceSource != RateSourceIncluded {
		t.Fatalf("expected INCLUDED_IN_LEASE, got %s", leaseEst.InsuranceSource)
	}
	if leaseEst.InsuranceCost != 0 || leaseEst.Legs[0].InsuranceCost != 0 {
		t.Fatalf("expected 0 insurance cost when included in lease, got %d", leaseEst.InsuranceCost)
	}
}

func TestIntegrationMaintenanceAmortizationAndOdometer(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "maint_test@example.com")
	loc := time.UTC
	base := time.Date(2024, time.March, 1, 10, 0, 0, 0, loc)

	// 1. Odometer resolution tests
	mustDrive(t, repo, v.ID, 101, base.Add(-10*24*time.Hour), 48000, 50)
	mustDrive(t, repo, v.ID, 102, base, 48500, 100)

	odo1, _, err := repo.GetOdometerAtDate(ctx, v.ID, base.Add(-5*24*time.Hour))
	if err != nil || odo1 != 48050 {
		t.Fatalf("expected 48050 odometer before second drive, got %v (err %v)", odo1, err)
	}
	odo2, _, err := repo.GetOdometerAtDate(ctx, v.ID, base.Add(2*time.Hour))
	if err != nil || odo2 != 48600 {
		t.Fatalf("expected 48600 odometer after second drive, got %v (err %v)", odo2, err)
	}

	// 2. Create maintenance expenses:
	// m1: 50,000 km revision (500 €) amortized over 50,000 km
	covKm := 50000.0
	covMonths := 24
	m1 := &models.MaintenanceExpense{
		VehicleID:        v.ID,
		Category:         "MAINTENANCE",
		Amount:           50000, // 500.00 €
		Currency:         "EUR",
		Date:             base,
		Odometer:         &odo2,
		AmortizationMode: "DISTANCE",
		CoverageKm:       &covKm,
		Description:      "Grande Révision 50k",
	}
	if err := repo.CreateMaintenanceExpense(ctx, m1); err != nil {
		t.Fatal(err)
	}

	// m2: Wiper blades (40 €) immediate (NONE)
	m2 := &models.MaintenanceExpense{
		VehicleID:        v.ID,
		Category:         "MAINTENANCE",
		Amount:           4000, // 40.00 €
		Currency:         "EUR",
		Date:             base.Add(35 * 24 * time.Hour), // April 2024
		AmortizationMode: "NONE",
		Description:      "Balais essuie-glaces",
	}
	if err := repo.CreateMaintenanceExpense(ctx, m2); err != nil {
		t.Fatal(err)
	}

	// m3: Revision 80,000 km (600 €) amortized over 50,000 km which closes m1
	m3 := &models.MaintenanceExpense{
		VehicleID:            v.ID,
		Category:             "MAINTENANCE",
		Amount:               60000, // 600.00 €
		Currency:             "EUR",
		Date:                 base.Add(180 * 24 * time.Hour), // September 2024
		AmortizationMode:     "DISTANCE",
		CoverageKm:           &covKm,
		CoverageMonths:       &covMonths,
		ClosesMaintenanceID: &m1.ID,
		Description:          "Révision 80k",
	}
	if err := repo.CreateMaintenanceExpense(ctx, m3); err != nil {
		t.Fatal(err)
	}

	list, err := repo.ListMaintenanceExpenses(ctx, v.ID)
	if err != nil || len(list) != 3 {
		t.Fatalf("expected 3 maintenance expenses, got %d (err %v)", len(list), err)
	}

	tcoSvc := NewTCOService(db.Pool, "UTC")
	monthlyDistances := map[string]float64{
		"2024-03": 1000,
		"2024-04": 2000,
		"2024-05": 2000,
		"2024-09": 1500,
	}
	now := base.Add(200 * 24 * time.Hour)
	maintMap, err := tcoSvc.computeMonthlyMaintenanceAmortization(ctx, v.ID, monthlyDistances, now)
	if err != nil {
		t.Fatalf("computeMonthlyMaintenanceAmortization failed: %v", err)
	}

	// March 2024: 1000 km * (50000 / 50000) = 1000 cents (10.00 €)
	if maintMap["2024-03"] != 1000 {
		t.Fatalf("expected 1000 cents in 2024-03, got %d", maintMap["2024-03"])
	}

	// April 2024: 2000 km * 1.00 cent/km = 2000 cents + m2 (4000 cents) = 6000 cents (60.00 €)
	if maintMap["2024-04"] != 6000 {
		t.Fatalf("expected 6000 cents in 2024-04, got %d", maintMap["2024-04"])
	}

	// September 2024: m1 was closed by m3, so m1 stops! m3 starts: 1500 km * (60000 / 50000 = 1.20) = 1800 cents
	if maintMap["2024-09"] != 1800 {
		t.Fatalf("expected 1800 cents in 2024-09, got %d", maintMap["2024-09"])
	}
}

func TestExpenseDocumentsIntegration(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	_ = db
	v := mustVehicle(t, repo, "user_doc@example.com")

	// 1. Create a document
	pdfData := []byte("%PDF-1.4 test document content for invoice")
	desc := "Facture Révision Tesla Chambourcy"
	doc := &models.ExpenseDocument{
		UserID:      v.UserID,
		VehicleID:   v.ID,
		Filename:    "facture_chambourcy.pdf",
		MimeType:    "application/pdf",
		FileSize:    int64(len(pdfData)),
		Data:        pdfData,
		Description: &desc,
	}
	if err := repo.SaveExpenseDocument(ctx, doc); err != nil {
		t.Fatalf("SaveExpenseDocument failed: %v", err)
	}
	if doc.ID == "" {
		t.Fatal("expected generated doc ID")
	}

	// 2. Fetch document by ID
	fetched, err := repo.GetExpenseDocumentByID(ctx, doc.ID, v.ID, v.UserID)
	if err != nil {
		t.Fatalf("GetExpenseDocumentByID failed: %v", err)
	}
	if fetched.Filename != "facture_chambourcy.pdf" || string(fetched.Data) != string(pdfData) {
		t.Fatalf("fetched doc mismatch: %+v", fetched)
	}

	// 3. Link document to 2 drive expenses (many-to-one)
	de1 := &models.DriveExpense{
		VehicleID:  v.ID,
		Type:       "TOLL",
		Amount:     1250,
		Currency:   "EUR",
		Date:       time.Now(),
		DocumentID: &doc.ID,
	}
	if err := repo.SaveDriveExpense(ctx, de1, nil, ""); err != nil {
		t.Fatalf("SaveDriveExpense de1 failed: %v", err)
	}
	if de1.DocumentFilename == nil || *de1.DocumentFilename != "facture_chambourcy.pdf" {
		t.Fatalf("expected DocumentFilename to be populated, got %v", de1.DocumentFilename)
	}

	de2 := &models.DriveExpense{
		VehicleID:  v.ID,
		Type:       "TOLL",
		Amount:     850,
		Currency:   "EUR",
		Date:       time.Now(),
		DocumentID: &doc.ID,
	}
	if err := repo.SaveDriveExpense(ctx, de2, nil, ""); err != nil {
		t.Fatalf("SaveDriveExpense de2 failed: %v", err)
	}

	// 4. Link document to 1 maintenance expense
	m := &models.MaintenanceExpense{
		VehicleID:        v.ID,
		Category:         "MAINTENANCE",
		Amount:           35000,
		Currency:         "EUR",
		Date:             time.Now(),
		Description:      "Révision complète",
		AmortizationMode: "DISTANCE",
		DocumentID:       &doc.ID,
	}
	if err := repo.CreateMaintenanceExpense(ctx, m); err != nil {
		t.Fatalf("CreateMaintenanceExpense failed: %v", err)
	}
	if m.DocumentFilename == nil || *m.DocumentFilename != "facture_chambourcy.pdf" {
		t.Fatalf("expected DocumentFilename to be populated, got %v", m.DocumentFilename)
	}

	// 5. Link document to 1 charge log
	chargeCost := money.Cents(1850)
	c := &models.ChargeLog{
		VehicleID:  v.ID,
		Date:       time.Now(),
		KwhAdded:   45.2,
		Cost:       &chargeCost,
		Currency:   "EUR",
		DocumentID: &doc.ID,
	}
	if err := repo.CreateManualCharge(ctx, c); err != nil {
		t.Fatalf("CreateManualCharge failed: %v", err)
	}
	if c.DocumentFilename == nil || *c.DocumentFilename != "facture_chambourcy.pdf" {
		t.Fatalf("expected DocumentFilename to be populated, got %v", c.DocumentFilename)
	}

	// 6. List documents - verify linked expenses count = 4
	docs, err := repo.ListExpenseDocuments(ctx, v.ID, v.UserID)
	if err != nil {
		t.Fatalf("ListExpenseDocuments failed: %v", err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}
	if docs[0].LinkedExpensesCount != 4 {
		t.Fatalf("expected LinkedExpensesCount=4 (2 tolls + 1 maint + 1 charge), got %d", docs[0].LinkedExpensesCount)
	}

	// 7. Verify listing includes DocumentID and DocumentFilename
	tolls, err := repo.ListDriveExpenses(ctx, v.ID)
	if err != nil || len(tolls) != 2 {
		t.Fatalf("expected 2 tolls, got %d (err %v)", len(tolls), err)
	}
	for _, toll := range tolls {
		if toll.DocumentID == nil || *toll.DocumentID != doc.ID {
			t.Fatalf("expected toll DocumentID %s, got %v", doc.ID, toll.DocumentID)
		}
		if toll.DocumentFilename == nil || *toll.DocumentFilename != "facture_chambourcy.pdf" {
			t.Fatalf("expected toll DocumentFilename 'facture_chambourcy.pdf', got %v", toll.DocumentFilename)
		}
	}

	maints, err := repo.ListMaintenanceExpenses(ctx, v.ID)
	if err != nil || len(maints) != 1 {
		t.Fatalf("expected 1 maintenance, got %d (err %v)", len(maints), err)
	}
	if maints[0].DocumentID == nil || *maints[0].DocumentID != doc.ID {
		t.Fatalf("expected maintenance DocumentID %s, got %v", doc.ID, maints[0].DocumentID)
	}

	charges, _, err := repo.ListCharges(ctx, v.ID, false, 50, 0)
	if err != nil || len(charges) != 1 {
		t.Fatalf("expected 1 charge, got %d (err %v)", len(charges), err)
	}
	if charges[0].DocumentID == nil || *charges[0].DocumentID != doc.ID {
		t.Fatalf("expected charge DocumentID %s, got %v", doc.ID, charges[0].DocumentID)
	}

	// 8. Delete document and verify ON DELETE SET NULL on all linked expenses
	if err := repo.DeleteExpenseDocument(ctx, doc.ID, v.ID, v.UserID); err != nil {
		t.Fatalf("DeleteExpenseDocument failed: %v", err)
	}

	tollsAfter, err := repo.ListDriveExpenses(ctx, v.ID)
	if err != nil || len(tollsAfter) != 2 {
		t.Fatalf("expected tolls to be preserved, got %d (err %v)", len(tollsAfter), err)
	}
	for _, toll := range tollsAfter {
		if toll.DocumentID != nil {
			t.Fatalf("expected toll DocumentID to be cleared to nil, got %v", *toll.DocumentID)
		}
	}

	maintsAfter, err := repo.ListMaintenanceExpenses(ctx, v.ID)
	if err != nil || len(maintsAfter) != 1 {
		t.Fatalf("expected maintenance to be preserved, got %d (err %v)", len(maintsAfter), err)
	}
	if maintsAfter[0].DocumentID != nil {
		t.Fatalf("expected maintenance DocumentID to be cleared to nil, got %v", *maintsAfter[0].DocumentID)
	}

	chargesAfter, _, err := repo.ListCharges(ctx, v.ID, false, 50, 0)
	if err != nil || len(chargesAfter) != 1 {
		t.Fatalf("expected charge to be preserved, got %d (err %v)", len(chargesAfter), err)
	}
	if chargesAfter[0].DocumentID != nil {
		t.Fatalf("expected charge DocumentID to be cleared to nil, got %v", *chargesAfter[0].DocumentID)
	}
}

