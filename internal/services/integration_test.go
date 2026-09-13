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

	// Vehicle-level insurance only: pro rata, flagged.
	annual := money.Cents(73050)
	v.AnnualInsuranceCost = &annual
	if err := repo.UpdateVehicle(ctx, v); err != nil {
		t.Fatal(err)
	}
	sum, err := tco.ComputeVehicleTCO(ctx, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sum.InsuranceSource != InsuranceSourceVehicleSettings || sum.InsuranceCost <= 0 {
		t.Fatalf("expected pro-rata insurance from vehicle settings, got %s %s", sum.InsuranceSource, sum.InsuranceCost)
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
	acq, price, bonus, resale, months, odo := "PURCHASE", money.Cents(4500000), money.Cents(500000), money.Cents(2000000), 48, 0.0
	v.AcquisitionType, v.PurchasePrice, v.PurchaseIncentives, v.ExpectedResaleValue = &acq, &price, &bonus, &resale
	v.PurchaseDate, v.ExpectedHoldingMonths, v.PurchaseOdometer, v.CurrentOdometer = &purchase, &months, &odo, 30000
	if err := repo.UpdateVehicle(ctx, v); err != nil {
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
