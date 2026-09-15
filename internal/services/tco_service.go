package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// Cost ledger categories (see the cost_ledger view).
const (
	LedgerEnergy      = "ENERGY"
	LedgerToll        = "TOLL"
	LedgerParking     = "PARKING"
	LedgerTravelOther = "TRAVEL_OTHER"
	LedgerTires       = "TIRES"
	LedgerMaintenance = "MAINTENANCE"
	LedgerRepair      = "REPAIR"
	LedgerInsurance   = "INSURANCE"
	LedgerFinancing   = "FINANCING"
	LedgerTax         = "TAX"
	LedgerSubscr      = "SUBSCRIPTION"
	LedgerAcquisition = "ACQUISITION"
)

// Insurance sources.
const (
	InsuranceSourceRecordedExpenses = "RECORDED_EXPENSES"
	InsuranceSourceIncluded         = "INCLUDED_IN_LEASE"
	InsuranceSourceInsufficientKm   = "INSUFFICIENT_DISTANCE"
	InsuranceSourceNone             = "NONE"
)

// MonthlyCost represents monthly expenditure (cash basis, acquisition excluded) and mileage.
type MonthlyCost struct {
	Month             string      `json:"month"` // YYYY-MM
	DistanceKm        float64     `json:"distance_km"` // Total effective distance (tracked + smoothed)
	TrackedDistanceKm float64     `json:"tracked_distance_km"` // Exact GPS drives distance
	SmoothedKm        float64     `json:"smoothed_km"` // Linearly smoothed / interpolated distance
	Energy            money.Cents `json:"energy"`
	Tolls             money.Cents `json:"tolls"`
	Maintenance       money.Cents `json:"maintenance"`
	Insurance         money.Cents `json:"insurance"`
	Financing         money.Cents `json:"financing"`
	Other             money.Cents `json:"other"` // Subscriptions, taxes, accessories, other
	Tires             money.Cents `json:"tires"` // Cash basis: full price in the purchase month
	TiresAmortized    money.Cents `json:"tires_amortized"` // Prorated by km driven while mounted, used for cost_per_km
	Total             money.Cents `json:"total"`
	CostPerKm         float64     `json:"cost_per_km"`
}

// TagCostBreakdown represents costs split by tag (e.g. Pro vs Perso).
type TagCostBreakdown struct {
	Tag         string      `json:"tag"`
	DistanceKm  float64     `json:"distance_km"`
	EnergyKwh   float64     `json:"energy_kwh"`
	TollsAmount money.Cents `json:"tolls_amount"`
	Percentage  float64     `json:"percentage"`
}

// TCOCompleteness lists the known gaps of the TCO figures.
type TCOCompleteness struct {
	IsComplete          bool     `json:"is_complete"`
	ChargesWithoutCost  int      `json:"charges_without_cost"`
	KwhWithoutCost      float64  `json:"kwh_without_cost"`
	UnconvertedExpenses int      `json:"unconverted_expenses"`
	UnqualifiedDrives   int      `json:"unqualified_drives"`
	UntrackedDistanceKm float64  `json:"untracked_distance_km"`
	OdometerGaps        int      `json:"odometer_gaps"`
	OdometerAnomalies   int      `json:"odometer_anomalies"`
	InsuranceMissing    bool     `json:"insurance_missing"`
	AcquisitionMissing  bool     `json:"acquisition_missing"`
	Warnings            []string `json:"warnings"`
	// ScorePct is a weighted completeness score (0-100) over the dimensions below.
	ScorePct   int                     `json:"score_pct"`
	Dimensions []CompletenessDimension `json:"dimensions"`
}

// CompletenessDimension is one weighted component of the completeness score.
type CompletenessDimension struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	ScorePct int     `json:"score_pct"`
	Weight   float64 `json:"weight"`
}

// completenessInputs gathers the ratios used by the completeness score.
type completenessInputs struct {
	kwhAdded, kwhPriced float64
	highwayDrives       int
	unqualifiedDrives   int
	trackedKm, basisKm  float64
	insurancePresent    bool
	acquisitionComplete bool
	pricedEntries       int
	unconvertedEntries  int
	drivesWithOdometer  int
	odometerAnomalies   int
}

func ratio(part, total float64) float64 {
	if total <= 0 {
		return 1
	}
	return math.Max(0, math.Min(1, part/total))
}

func boolScore(ok bool) float64 {
	if ok {
		return 1
	}
	return 0
}

// completenessScore weights how much of the TCO rests on complete data.
func completenessScore(in completenessInputs) (int, []CompletenessDimension) {
	distance := 0.0
	if in.basisKm > 0 {
		distance = ratio(in.trackedKm, in.basisKm)
	}
	dims := []struct {
		key, label string
		weight     float64
		score      float64
	}{
		{"energy", "Recharges avec coût (kWh)", 0.30, ratio(in.kwhPriced, in.kwhAdded)},
		{"distance", "Kilomètres couverts par des trajets", 0.20, distance},
		{"tolls", "Trajets autoroutiers qualifiés", 0.15, 1 - ratio(float64(in.unqualifiedDrives), float64(in.highwayDrives))},
		{"insurance", "Assurance renseignée", 0.10, boolScore(in.insurancePresent)},
		{"acquisition", "Acquisition et décote renseignées", 0.10, boolScore(in.acquisitionComplete)},
		{"odometer", "Continuité de l'odomètre", 0.10, 1 - ratio(float64(in.odometerAnomalies), float64(in.drivesWithOdometer))},
		{"currency", "Dépenses converties en euros", 0.05, 1 - ratio(float64(in.unconvertedEntries), float64(in.pricedEntries+in.unconvertedEntries))},
	}
	var total float64
	out := make([]CompletenessDimension, 0, len(dims))
	for _, d := range dims {
		total += d.weight * d.score
		out = append(out, CompletenessDimension{Key: d.key, Label: d.label, ScorePct: int(math.Round(d.score * 100)), Weight: d.weight})
	}
	return int(math.Round(total * 100)), out
}

// TCOSummary represents the global TCO calculation, built from the cost_ledger view.
//
//   - TotalCost: running costs actually paid (tires at purchase), acquisition excluded.
//   - FullCost: economic cost of ownership: running costs with tires amortized per km, plus depreciation.
//   - Per-km figures use DistanceBasisKm: the largest of the distance tracked by drives, the odometer span
//     of those drives and the distance driven since acquisition.
//   - UsageCostPerKm is the marginal cost of driving (energy + tolls/parking).
type TCOSummary struct {
	TotalDistanceKm    float64     `json:"total_distance_km"`
	OdometerDistanceKm float64     `json:"odometer_distance_km"`
	SmoothedDistanceKm float64     `json:"smoothed_distance_km"`
	DistanceBasisKm    float64     `json:"distance_basis_km"`
	TotalCost          money.Cents `json:"total_cost"`
	TotalCostPerKm     float64     `json:"total_cost_per_km"`
	UsageCostPerKm     float64     `json:"usage_cost_per_km"`
	FullCost           money.Cents `json:"full_cost"`
	FullCostPerKm      float64     `json:"full_cost_per_km"`

	AcquisitionType       string      `json:"acquisition_type"` // CASH | LOAN | LOA | LLD
	AcquisitionCost       money.Cents `json:"acquisition_cost"` // Purchase price and fees net of incentives, exercised LOA option
	DepreciationCost      money.Cents `json:"depreciation_cost"`
	DepreciationCostPerKm float64     `json:"depreciation_cost_per_km"`
	ContractEndDate       *time.Time  `json:"contract_end_date,omitempty"`  // Lease term
	OwnershipEndDate      *time.Time  `json:"ownership_end_date,omitempty"` // Sale or return

	LeaseExcessKmCost      money.Cents `json:"lease_excess_km_cost"`      // Accrued against the pro-rata allowance
	LeaseExcessKmProjected money.Cents `json:"lease_excess_km_projected"` // Expected at contract end at the current pace
	LeaseKmDriven          float64     `json:"lease_km_driven"`
	LeaseKmAllowanceToDate float64     `json:"lease_km_allowance_to_date"`

	CarpoolRevenue   money.Cents `json:"carpool_revenue"`
	FullCostNet      money.Cents `json:"full_cost_net"` // Full cost minus carpool revenue
	FullCostNetPerKm float64     `json:"full_cost_net_per_km"`

	EnergyCost      money.Cents `json:"energy_cost"`
	EnergyCostPerKm float64     `json:"energy_cost_per_km"`
	TotalKwhAdded   float64     `json:"total_kwh_added"`
	AvgCostPerKwh   float64     `json:"avg_cost_per_kwh"`

	TollsCost      money.Cents `json:"tolls_cost"` // Tolls, parking, ferries
	TollsCostPerKm float64     `json:"tolls_cost_per_km"`

	TiresCost               money.Cents `json:"tires_cost"`
	TiresCostPerKm          float64     `json:"tires_cost_per_km"`
	TiresAmortizedCost      money.Cents `json:"tires_amortized_cost"`
	TiresAmortizedCostPerKm float64     `json:"tires_amortized_cost_per_km"`

	MaintenanceCost      money.Cents `json:"maintenance_cost"`
	MaintenanceCostPerKm float64     `json:"maintenance_cost_per_km"`
	RepairCost           money.Cents `json:"repair_cost"` // Unplanned repairs, insurance deductibles
	RepairCostPerKm      float64     `json:"repair_cost_per_km"`

	InsuranceCost      money.Cents `json:"insurance_cost"`
	InsuranceCostPerKm float64     `json:"insurance_cost_per_km"`
	InsuranceSource    string      `json:"insurance_source"`

	FinancingCost      money.Cents `json:"financing_cost"`      // Cash: rents, down payment, fees, loan interest
	FinancingFullCost  money.Cents `json:"financing_full_cost"` // Prepaid amounts spread, return fees and excess mileage accrued
	FinancingCostPerKm float64     `json:"financing_cost_per_km"`

	SubscriptionCost money.Cents `json:"subscription_cost"`
	TaxCost          money.Cents `json:"tax_cost"`
	OtherCost        money.Cents `json:"other_cost"`
	OtherCostPerKm   float64     `json:"other_cost_per_km"` // Subscriptions + taxes + other

	TagBreakdown []TagCostBreakdown `json:"tag_breakdown"`
	MonthlyCosts []MonthlyCost      `json:"monthly_costs"`

	Completeness TCOCompleteness `json:"completeness"`
}

// TCOService computes TCO metrics from the cost ledger.
type TCOService struct {
	pool     *pgxpool.Pool
	repo     *database.Repository
	timezone string
}

// NewTCOService creates a new TCOService; timezone (IANA name) is used for monthly buckets.
func NewTCOService(pool *pgxpool.Pool, timezone string) *TCOService {
	if timezone == "" {
		timezone = "UTC"
	}
	return &TCOService{pool: pool, repo: database.NewRepository(pool), timezone: timezone}
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
func round1(v float64) float64 { return math.Round(v*10) / 10 }

func perKm(amount money.Cents, km float64) float64 {
	if km <= 0 {
		return 0
	}
	return round3(amount.Float() / km)
}

// ComputeVehicleTCO calculates complete TCO for a given vehicle.
func (s *TCOService) ComputeVehicleTCO(ctx context.Context, vehicleID string) (*TCOSummary, error) {
	sum := &TCOSummary{}
	comp := &sum.Completeness
	now := time.Now()

	// 1. Ownership contract and odometer
	ownership, err := s.repo.GetVehicleOwnership(ctx, vehicleID)
	if err != nil && !errors.Is(err, database.ErrNotFound) {
		return nil, fmt.Errorf("ownership: %w", err)
	}
	var currentOdometer float64
	if err := s.pool.QueryRow(ctx, `SELECT current_odometer FROM vehicles WHERE id = $1;`, vehicleID).Scan(&currentOdometer); err != nil {
		return nil, fmt.Errorf("vehicle: %w", err)
	}

	// 2. Distance: tracked drives, odometer span, distance since the start of the contract
	var trackedKm, odometerSpan, trackedSinceStart float64
	var contractStart *time.Time
	if ownership != nil {
		contractStart = &ownership.StartDate
	}
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(distance_km), 0),
		       COALESCE(MAX(end_odometer) FILTER (WHERE end_odometer > 0) - MIN(start_odometer) FILTER (WHERE start_odometer > 0), 0),
		       COALESCE(SUM(distance_km) FILTER (WHERE $2::timestamptz IS NULL OR start_time >= $2), 0)
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL;
	`, vehicleID, contractStart).Scan(&trackedKm, &odometerSpan, &trackedSinceStart); err != nil {
		return nil, fmt.Errorf("distance: %w", err)
	}
	basisKm := math.Max(trackedKm, odometerSpan)
	kmSinceStart := trackedSinceStart
	if ownership != nil && ownership.StartOdometer != nil && currentOdometer > *ownership.StartOdometer {
		kmSinceStart = math.Max(kmSinceStart, currentOdometer-*ownership.StartOdometer)
		basisKm = math.Max(basisKm, kmSinceStart)
	}
	if untracked := basisKm - trackedKm; untracked > 50 && untracked > 0.01*basisKm {
		comp.UntrackedDistanceKm = round1(untracked)
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%.0f km parcourus n'apparaissent dans aucun trajet (avant TeslaMate ou TeslaMate hors ligne) : le coût au km utilise la distance odométrique", untracked))
	}
	owned := ComputeOwnershipCosts(ownership, now, kmSinceStart)

	// 3. Ledger totals per category
	byCategory := map[string]money.Cents{}
	rows, err := s.pool.Query(ctx, `
		SELECT category, COALESCE(SUM(amount_eur), 0) FROM cost_ledger WHERE vehicle_id = $1 GROUP BY category;
	`, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("ledger: %w", err)
	}
	for rows.Next() {
		var category string
		var amount money.Cents
		if err := rows.Scan(&category, &amount); err != nil {
			rows.Close()
			return nil, err
		}
		byCategory[category] = amount
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 4. Energy volume and gaps
	var kwhAdded, kwhPriced float64
	var unconvertedCharges int
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(kwh_added), 0),
		       COALESCE(SUM(kwh_added) FILTER (WHERE cost IS NOT NULL AND (currency = 'EUR' OR fx_rate IS NOT NULL)), 0),
		       COUNT(*) FILTER (WHERE cost IS NULL),
		       COALESCE(SUM(kwh_added) FILTER (WHERE cost IS NULL), 0),
		       COUNT(*) FILTER (WHERE cost IS NOT NULL AND currency <> 'EUR' AND fx_rate IS NULL)
		FROM charge_logs
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL;
	`, vehicleID).Scan(&kwhAdded, &kwhPriced, &comp.ChargesWithoutCost, &comp.KwhWithoutCost, &unconvertedCharges); err != nil {
		return nil, fmt.Errorf("energy: %w", err)
	}
	if comp.ChargesWithoutCost > 0 {
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%d recharge(s) sans coût (%.0f kWh) : coût énergétique sous-estimé", comp.ChargesWithoutCost, comp.KwhWithoutCost))
	}

	// 5. Unconverted foreign amounts, insurance expenses, carpool revenue
	var unconvertedOther, insuranceEntries int
	if err := s.pool.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM drive_expenses WHERE vehicle_id = $1 AND currency <> 'EUR' AND fx_rate IS NULL)
		     + (SELECT COUNT(*) FROM maintenance_expenses WHERE vehicle_id = $1 AND currency <> 'EUR' AND fx_rate IS NULL),
		       (SELECT COUNT(*) FROM maintenance_expenses WHERE vehicle_id = $1 AND category = 'INSURANCE'),
		       (SELECT COALESCE(SUM(total_revenue), 0) FROM carpool_trips WHERE vehicle_id = $1);
	`, vehicleID).Scan(&unconvertedOther, &insuranceEntries, &sum.CarpoolRevenue); err != nil {
		return nil, fmt.Errorf("completeness: %w", err)
	}
	comp.UnconvertedExpenses = unconvertedCharges + unconvertedOther
	if comp.UnconvertedExpenses > 0 {
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%d dépense(s) en devise étrangère sans taux de conversion : exclues des totaux", comp.UnconvertedExpenses))
	}
	switch {
	case insuranceEntries > 0:
		sum.InsuranceSource = InsuranceSourceRecordedExpenses
	case owned.IncludesInsurance:
		sum.InsuranceSource = InsuranceSourceIncluded
	default:
		sum.InsuranceSource = InsuranceSourceNone
		comp.InsuranceMissing = true
		comp.Warnings = append(comp.Warnings, "Aucune prime d'assurance enregistrée (dépense récurrente « Assurance »)")
	}

	// 6. Tires amortized by kilometers actually driven on each tire
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(
		           CASE
		               WHEN t.current_position = 'DISPOSED' OR t.is_archived THEN t.purchase_price
		               ELSE t.purchase_price * LEAST(1.0, GREATEST(0,
		                   (t.accumulated_distance_km - t.initial_distance_km
		                    + CASE WHEN t.current_position IN ('FL', 'FR', 'RL', 'RR') AND t.mounted_odometer IS NOT NULL
		                           THEN GREATEST(v.current_odometer - t.mounted_odometer, 0) ELSE 0 END)
		                   / GREATEST(t.estimated_lifespan_km - t.initial_distance_km, 1)))
		           END
		       ), 0)
		FROM tires t
		JOIN vehicles v ON v.id = t.vehicle_id
		WHERE t.vehicle_id = $1;
	`, vehicleID).Scan(&sum.TiresAmortizedCost); err != nil {
		return nil, fmt.Errorf("tires: %w", err)
	}

	// 7. Acquisition, financing and depreciation
	if ownership != nil {
		sum.AcquisitionType = ownership.AcquisitionType
		sum.ContractEndDate = owned.ContractEndDate
		sum.OwnershipEndDate = ownership.EndDate
	}
	sum.AcquisitionCost = byCategory[LedgerAcquisition]
	sum.DepreciationCost = owned.Depreciation
	sum.LeaseExcessKmCost = owned.LeaseExcessKm
	sum.LeaseExcessKmProjected = owned.LeaseExcessKmProjected
	sum.LeaseKmDriven = round1(owned.LeaseKmDriven)
	sum.LeaseKmAllowanceToDate = round1(owned.LeaseKmAllowanceToDate)
	if len(owned.Missing) > 0 {
		comp.AcquisitionMissing = true
		comp.Warnings = append(comp.Warnings, owned.Missing...)
	}
	comp.Warnings = append(comp.Warnings, owned.LeaseWarnings(now)...)

	// 8. Toll qualification backlog
	var highwayDrives, drivesWithOdometer, ledgerEntries int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE `+database.UnqualifiedDrivePredicate+`),
		       COUNT(*) FILTER (WHERE `+database.HighwayDrivePredicate+`),
		       COUNT(*) FILTER (WHERE start_odometer > 0 AND end_odometer > 0),
		       (SELECT COUNT(*) FROM cost_ledger WHERE vehicle_id = $1)
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL;
	`, vehicleID).Scan(&comp.UnqualifiedDrives, &highwayDrives, &drivesWithOdometer, &ledgerEntries); err != nil {
		return nil, fmt.Errorf("unqualified drives: %w", err)
	}
	if comp.UnqualifiedDrives > 0 {
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%d trajet(s) de type autoroutier sans péage renseigné ni qualification « sans péage »", comp.UnqualifiedDrives))
	}
	if basisKm <= 0 {
		comp.Warnings = append(comp.Warnings, "Aucun kilométrage enregistré : le coût au kilomètre ne peut pas être calculé")
	}

	// 9. Odometer continuity
	var gapKm float64
	if err := s.pool.QueryRow(ctx, database.OdometerContinuitySummarySQL, vehicleID).Scan(&comp.OdometerGaps, &gapKm, &comp.OdometerAnomalies); err != nil {
		return nil, fmt.Errorf("odometer continuity: %w", err)
	}
	if comp.OdometerGaps > 0 {
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%d trou(s) d'odomètre entre trajets consécutifs (%.0f km sans trajet enregistré)", comp.OdometerGaps, gapKm))
	}
	if comp.OdometerAnomalies > 0 {
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%d incohérence(s) d'odomètre (odomètre en recul ou distance différente du relevé) à vérifier dans TeslaMate", comp.OdometerAnomalies))
	}

	comp.IsComplete = len(comp.Warnings) == 0
	if comp.Warnings == nil {
		comp.Warnings = []string{}
	}
	comp.ScorePct, comp.Dimensions = completenessScore(completenessInputs{
		kwhAdded:            kwhAdded,
		kwhPriced:           kwhPriced,
		highwayDrives:       highwayDrives,
		unqualifiedDrives:   comp.UnqualifiedDrives,
		trackedKm:           trackedKm,
		basisKm:             basisKm,
		insurancePresent:    !comp.InsuranceMissing,
		acquisitionComplete: !comp.AcquisitionMissing,
		pricedEntries:       ledgerEntries,
		unconvertedEntries:  comp.UnconvertedExpenses,
		drivesWithOdometer:  drivesWithOdometer,
		odometerAnomalies:   comp.OdometerAnomalies + comp.OdometerGaps,
	})

	// 10. Aggregates
	energy := byCategory[LedgerEnergy]
	travel := byCategory[LedgerToll] + byCategory[LedgerParking] + byCategory[LedgerTravelOther]
	tires := byCategory[LedgerTires]
	maintenance := byCategory[LedgerMaintenance]
	repair := byCategory[LedgerRepair]
	insurance := byCategory[LedgerInsurance]
	financing := byCategory[LedgerFinancing]
	subscription, tax := byCategory[LedgerSubscr], byCategory[LedgerTax]
	var other money.Cents
	for category, amount := range byCategory {
		switch category {
		case LedgerEnergy, LedgerToll, LedgerParking, LedgerTravelOther, LedgerTires, LedgerMaintenance, LedgerRepair,
			LedgerInsurance, LedgerFinancing, LedgerTax, LedgerSubscr, LedgerAcquisition:
		default:
			other += amount
		}
	}
	otherTotal := subscription + tax + other

	running := energy + travel + maintenance + repair + insurance + financing + otherTotal
	sum.TotalCost = running + tires
	// Economic financing cost: prepaid lease amounts spread over the contract, return fees and excess mileage accrued
	sum.FinancingFullCost = financing + owned.LeasePrepaidAdjustment + owned.LeaseEndFeesAdjustment + owned.LeaseExcessKm
	sum.FullCost = running - financing + sum.FinancingFullCost + sum.TiresAmortizedCost + sum.DepreciationCost
	sum.FullCostNet = sum.FullCost - sum.CarpoolRevenue

	sum.TotalDistanceKm = round1(trackedKm)
	sum.OdometerDistanceKm = round1(odometerSpan)
	sum.DistanceBasisKm = round1(basisKm)
	sum.TotalCostPerKm = perKm(sum.TotalCost, basisKm)
	sum.UsageCostPerKm = perKm(energy+travel, basisKm)
	sum.FullCostPerKm = perKm(sum.FullCost, basisKm)
	sum.FullCostNetPerKm = perKm(sum.FullCostNet, basisKm)
	sum.DepreciationCostPerKm = perKm(sum.DepreciationCost, basisKm)
	sum.EnergyCost = energy
	sum.EnergyCostPerKm = perKm(energy, basisKm)
	sum.TotalKwhAdded = round1(kwhAdded)
	if kwhPriced > 0 {
		sum.AvgCostPerKwh = round3(energy.Float() / kwhPriced)
	}
	sum.TollsCost = travel
	sum.TollsCostPerKm = perKm(travel, basisKm)
	sum.TiresCost = tires
	sum.TiresCostPerKm = perKm(tires, basisKm)
	sum.TiresAmortizedCostPerKm = perKm(sum.TiresAmortizedCost, basisKm)
	sum.MaintenanceCost = maintenance
	sum.MaintenanceCostPerKm = perKm(maintenance, basisKm)
	sum.RepairCost = repair
	sum.RepairCostPerKm = perKm(repair, basisKm)
	sum.InsuranceCost = insurance
	sum.InsuranceCostPerKm = perKm(insurance, basisKm)
	sum.FinancingCost = financing
	sum.FinancingCostPerKm = perKm(sum.FinancingFullCost, basisKm)
	sum.SubscriptionCost = subscription
	sum.TaxCost = tax
	sum.OtherCost = other
	sum.OtherCostPerKm = perKm(otherTotal, basisKm)

	if sum.TagBreakdown, err = s.tagBreakdown(ctx, vehicleID, trackedKm); err != nil {
		return nil, fmt.Errorf("tag breakdown: %w", err)
	}
	if sum.MonthlyCosts, sum.SmoothedDistanceKm, err = s.monthlyCosts(ctx, vehicleID, ownership, currentOdometer, now); err != nil {
		return nil, fmt.Errorf("monthly costs: %w", err)
	}
	if sum.SmoothedDistanceKm > 0 {
		basisKm = math.Max(basisKm, trackedKm+sum.SmoothedDistanceKm)
		sum.DistanceBasisKm = round1(basisKm)
		sum.TotalCostPerKm = perKm(sum.TotalCost, basisKm)
		sum.UsageCostPerKm = perKm(energy+travel, basisKm)
		sum.FullCostPerKm = perKm(sum.FullCost, basisKm)
		sum.FullCostNetPerKm = perKm(sum.FullCostNet, basisKm)
		sum.DepreciationCostPerKm = perKm(sum.DepreciationCost, basisKm)
		sum.EnergyCostPerKm = perKm(energy, basisKm)
		sum.TollsCostPerKm = perKm(travel, basisKm)
		sum.TiresCostPerKm = perKm(tires, basisKm)
		sum.TiresAmortizedCostPerKm = perKm(sum.TiresAmortizedCost, basisKm)
		sum.MaintenanceCostPerKm = perKm(maintenance, basisKm)
		sum.RepairCostPerKm = perKm(repair, basisKm)
		sum.InsuranceCostPerKm = perKm(insurance, basisKm)
		sum.FinancingCostPerKm = perKm(sum.FinancingFullCost, basisKm)
		sum.OtherCostPerKm = perKm(otherTotal, basisKm)
	}
	return sum, nil
}

func (s *TCOService) tagBreakdown(ctx context.Context, vehicleID string, totalDistance float64) ([]TagCostBreakdown, error) {
	rows, err := s.pool.Query(ctx, database.DriveTollAllocationCTE+`,
		per_drive AS (
			SELECT drive_id, SUM(allocated) AS amount FROM allocations GROUP BY drive_id
		)
		SELECT sub.tag,
		       COALESCE(SUM(sub.distance_km), 0),
		       COALESCE(SUM(sub.energy_consumed_kwh), 0),
		       COALESCE(SUM(pd.amount), 0)
		FROM (
			SELECT d.id,
			       unnest(CASE WHEN d.tags = '{}' OR d.tags IS NULL THEN ARRAY['Non tagué'] ELSE d.tags END) AS tag,
			       d.distance_km, d.energy_consumed_kwh
			FROM drives d
			WHERE d.vehicle_id = $1 AND d.deleted_upstream_at IS NULL
		) sub
		LEFT JOIN per_drive pd ON pd.drive_id = sub.id
		GROUP BY sub.tag
		ORDER BY 2 DESC;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []TagCostBreakdown{}
	for rows.Next() {
		var tb TagCostBreakdown
		if err := rows.Scan(&tb.Tag, &tb.DistanceKm, &tb.EnergyKwh, &tb.TollsAmount); err != nil {
			return nil, err
		}
		if totalDistance > 0 {
			tb.Percentage = round1(tb.DistanceKm / totalDistance * 100)
		}
		tb.DistanceKm = round1(tb.DistanceKm)
		tb.EnergyKwh = round1(tb.EnergyKwh)
		list = append(list, tb)
	}
	return list, rows.Err()
}

// computeMileageSmoothing interpolates missing mileage across months between known odometer checkpoints.
func (s *TCOService) computeMileageSmoothing(ctx context.Context, vehicleID string, ownership *models.VehicleOwnership, currentOdometer float64, now time.Time) (map[string]float64, error) {
	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		loc = time.UTC
	}

	checkpoints, err := s.repo.ListOdometerCheckpoints(ctx, vehicleID)
	if err != nil {
		return nil, err
	}

	type odoPoint struct {
		date time.Time
		odo  float64
	}

	var points []odoPoint
	if ownership != nil && ownership.StartOdometer != nil && *ownership.StartOdometer > 0 {
		points = append(points, odoPoint{
			date: ownership.StartDate.In(loc),
			odo:  *ownership.StartOdometer,
		})
	}
	for _, cp := range checkpoints {
		points = append(points, odoPoint{
			date: cp.Date.In(loc),
			odo:  cp.Odometer,
		})
	}
	if currentOdometer > 0 {
		points = append(points, odoPoint{
			date: now.In(loc),
			odo:  currentOdometer,
		})
	}

	if len(points) < 2 {
		return nil, nil
	}

	// Sort chronologically by date
	sort.Slice(points, func(i, j int) bool {
		if points[i].date.Equal(points[j].date) {
			return points[i].odo < points[j].odo
		}
		return points[i].date.Before(points[j].date)
	})

	// Deduplicate by date (keep highest odometer on same day) and eliminate regressions
	var cleanPoints []odoPoint
	for _, p := range points {
		if len(cleanPoints) == 0 {
			cleanPoints = append(cleanPoints, p)
			continue
		}
		last := &cleanPoints[len(cleanPoints)-1]
		if last.date.Format("2006-01-02") == p.date.Format("2006-01-02") {
			if p.odo > last.odo {
				last.odo = p.odo
			}
			continue
		}
		if p.odo >= last.odo {
			cleanPoints = append(cleanPoints, p)
		}
	}

	if len(cleanPoints) < 2 {
		return nil, nil
	}

	smoothedByMonth := make(map[string]float64)

	for i := 0; i < len(cleanPoints)-1; i++ {
		p1 := cleanPoints[i]
		p2 := cleanPoints[i+1]

		deltaOdo := p2.odo - p1.odo
		if deltaOdo <= 0 {
			continue
		}

		t1 := p1.date
		t2 := p2.date
		if !t2.After(t1) {
			continue
		}

		// Calculate tracked distance in [t1, t2]
		var trackedKm float64
		err := s.pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(distance_km), 0)
			FROM drives
			WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL
			  AND start_time >= $2 AND start_time < $3;
		`, vehicleID, t1, t2).Scan(&trackedKm)
		if err != nil {
			return nil, err
		}

		missingKm := deltaOdo - trackedKm
		if missingKm <= 1.0 {
			continue
		}

		for month, km := range allocateMissingKmByMonth(t1, t2, missingKm) {
			smoothedByMonth[month] += km
		}
	}

	return smoothedByMonth, nil
}

// computeMonthlyTireAmortization prorates each tire's purchase price across the months it was
// actually driven on, mirroring the lifetime formula used for TiresAmortizedCost (km used /
// estimated lifespan) but as a month-by-month delta of the cumulative amortized amount. A tire
// disposed or archived before reaching 100% of its lifespan recognizes the remaining balance in
// the month it was last dismounted, matching the "fully consumed at disposal" rule used overall.
func (s *TCOService) computeMonthlyTireAmortization(ctx context.Context, vehicleID string, now time.Time) (map[string]money.Cents, error) {
	type tireInfo struct {
		purchasePrice     money.Cents
		lifespanKm        float64
		disposed          bool
		lastDismountMonth string
	}
	tires := make(map[string]*tireInfo)

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, purchase_price, GREATEST(estimated_lifespan_km - initial_distance_km, 1),
		       (current_position = 'DISPOSED' OR is_archived)
		FROM tires
		WHERE vehicle_id = $1 AND purchase_price > 0;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var id string
		info := &tireInfo{}
		if err := rows.Scan(&id, &info.purchasePrice, &info.lifespanKm, &info.disposed); err != nil {
			rows.Close()
			return nil, err
		}
		tires[id] = info
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(tires) == 0 {
		return nil, nil
	}

	// Km driven by the vehicle while each tire was actually mounted, bucketed by month.
	kmRows, err := s.pool.Query(ctx, `
		SELECT s.tire_id::text, TO_CHAR(d.start_time AT TIME ZONE $3, 'YYYY-MM') AS m, SUM(d.distance_km)
		FROM tire_mount_sessions s
		JOIN drives d ON d.vehicle_id = s.vehicle_id
		    AND d.deleted_upstream_at IS NULL
		    AND d.start_time >= s.mounted_date
		    AND d.start_time < COALESCE(s.dismounted_date, $2)
		WHERE s.vehicle_id = $1
		GROUP BY s.tire_id, m;
	`, vehicleID, now, s.timezone)
	if err != nil {
		return nil, err
	}
	kmByTireMonth := make(map[string]map[string]float64)
	months := map[string]bool{}
	for kmRows.Next() {
		var tireID, month string
		var km float64
		if err := kmRows.Scan(&tireID, &month, &km); err != nil {
			kmRows.Close()
			return nil, err
		}
		if kmByTireMonth[tireID] == nil {
			kmByTireMonth[tireID] = map[string]float64{}
		}
		kmByTireMonth[tireID][month] = km
		months[month] = true
	}
	kmRows.Close()
	if err := kmRows.Err(); err != nil {
		return nil, err
	}

	// Last dismount month per tire, to book a disposed tire's remaining balance where it belongs.
	dismountRows, err := s.pool.Query(ctx, `
		SELECT tire_id::text, TO_CHAR(MAX(dismounted_date) AT TIME ZONE $2, 'YYYY-MM')
		FROM tire_mount_sessions
		WHERE vehicle_id = $1 AND dismounted_date IS NOT NULL
		GROUP BY tire_id;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, err
	}
	for dismountRows.Next() {
		var tireID, month string
		if err := dismountRows.Scan(&tireID, &month); err != nil {
			dismountRows.Close()
			return nil, err
		}
		if info, ok := tires[tireID]; ok {
			info.lastDismountMonth = month
		}
	}
	dismountRows.Close()
	if err := dismountRows.Err(); err != nil {
		return nil, err
	}

	sortedMonths := make([]string, 0, len(months))
	for m := range months {
		sortedMonths = append(sortedMonths, m)
	}
	sort.Strings(sortedMonths)

	result := make(map[string]money.Cents)
	for tireID, info := range tires {
		var cumKm float64
		var cumAmortized money.Cents
		for _, m := range sortedMonths {
			km := kmByTireMonth[tireID][m]
			if km <= 0 {
				continue
			}
			cumKm += km
			fraction := math.Min(1.0, cumKm/info.lifespanKm)
			newCum := money.Cents(math.Round(float64(info.purchasePrice) * fraction))
			if delta := newCum - cumAmortized; delta > 0 {
				result[m] += delta
				cumAmortized = newCum
			}
		}
		if info.disposed && cumAmortized < info.purchasePrice {
			month := info.lastDismountMonth
			if month == "" {
				month = now.Format("2006-01")
			}
			result[month] += info.purchasePrice - cumAmortized
		}
	}
	return result, nil
}

// monthlyCosts builds the cash-basis monthly timeline (acquisition excluded) in the reporting timezone,
// including linear smoothing of missing mileage between odometer checkpoints.
func (s *TCOService) monthlyCosts(ctx context.Context, vehicleID string, ownership *models.VehicleOwnership, currentOdometer float64, now time.Time) ([]MonthlyCost, float64, error) {
	monthlyMap := make(map[string]*MonthlyCost)
	get := func(m string) *MonthlyCost {
		if _, ok := monthlyMap[m]; !ok {
			monthlyMap[m] = &MonthlyCost{Month: m}
		}
		return monthlyMap[m]
	}

	rows, err := s.pool.Query(ctx, `
		SELECT TO_CHAR(entry_date AT TIME ZONE $2, 'YYYY-MM') AS m, category, SUM(amount_eur)
		FROM cost_ledger
		WHERE vehicle_id = $1 AND category <> 'ACQUISITION'
		GROUP BY m, category;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, 0, err
	}
	for rows.Next() {
		var m, category string
		var amount money.Cents
		if err := rows.Scan(&m, &category, &amount); err != nil {
			rows.Close()
			return nil, 0, err
		}
		mc := get(m)
		switch category {
		case LedgerEnergy:
			mc.Energy += amount
		case LedgerToll, LedgerParking, LedgerTravelOther:
			mc.Tolls += amount
		case LedgerTires:
			mc.Tires += amount
		case LedgerMaintenance, LedgerRepair:
			mc.Maintenance += amount
		case LedgerInsurance:
			mc.Insurance += amount
		case LedgerFinancing:
			mc.Financing += amount
		default:
			mc.Other += amount
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	distRows, err := s.pool.Query(ctx, `
		SELECT TO_CHAR(start_time AT TIME ZONE $2, 'YYYY-MM') AS m, COALESCE(SUM(distance_km), 0)
		FROM drives WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL GROUP BY m;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, 0, err
	}
	for distRows.Next() {
		var m string
		var km float64
		if err := distRows.Scan(&m, &km); err != nil {
			distRows.Close()
			return nil, 0, err
		}
		get(m).DistanceKm += km
	}
	distRows.Close()
	if err := distRows.Err(); err != nil {
		return nil, 0, err
	}

	// Compute smoothed missing distance between checkpoints
	smoothedMap, err := s.computeMileageSmoothing(ctx, vehicleID, ownership, currentOdometer, now)
	if err != nil {
		return nil, 0, err
	}
	for m, smoothed := range smoothedMap {
		if smoothed > 0 {
			get(m).SmoothedKm += round1(smoothed)
		}
	}

	// Tires: prorate each purchase by km driven while mounted instead of dumping the full
	// price into the purchase month, so a low-mileage purchase month doesn't spike cost/km.
	tireAmortMap, err := s.computeMonthlyTireAmortization(ctx, vehicleID, now)
	if err != nil {
		return nil, 0, err
	}
	for m, amount := range tireAmortMap {
		get(m).TiresAmortized += amount
	}

	var totalSmoothed float64
	monthlyCosts := make([]MonthlyCost, 0, len(monthlyMap))
	for _, mc := range monthlyMap {
		mc.TrackedDistanceKm = round1(mc.DistanceKm)
		mc.SmoothedKm = round1(mc.SmoothedKm)
		mc.DistanceKm = round1(mc.TrackedDistanceKm + mc.SmoothedKm)
		totalSmoothed += mc.SmoothedKm
		mc.Total = mc.Energy + mc.Tolls + mc.Maintenance + mc.Insurance + mc.Financing + mc.Other + mc.Tires
		costForPerKm := mc.Energy + mc.Tolls + mc.Maintenance + mc.Insurance + mc.Financing + mc.Other + mc.TiresAmortized
		mc.CostPerKm = perKm(costForPerKm, mc.DistanceKm)
		monthlyCosts = append(monthlyCosts, *mc)
	}
	sort.Slice(monthlyCosts, func(i, j int) bool {
		return monthlyCosts[i].Month < monthlyCosts[j].Month
	})

	if len(monthlyCosts) == 0 {
		monthlyCosts = append(monthlyCosts, MonthlyCost{Month: now.Format("2006-01")})
	}
	return monthlyCosts, round1(totalSmoothed), nil
}
