package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/database"
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
	LedgerInsurance   = "INSURANCE"
	LedgerFinancing   = "FINANCING"
	LedgerTax         = "TAX"
	LedgerSubscr      = "SUBSCRIPTION"
	LedgerAcquisition = "ACQUISITION"
)

// Insurance sources.
const (
	InsuranceSourceRecordedExpenses = "RECORDED_EXPENSES"
	InsuranceSourceVehicleSettings  = "VEHICLE_SETTINGS"
	InsuranceSourceNone             = "NONE"
	InsuranceSourceDefault          = "DEFAULT"
)

// MonthlyCost represents monthly expenditure (cash basis, acquisition excluded) and mileage.
type MonthlyCost struct {
	Month       string      `json:"month"` // YYYY-MM
	DistanceKm  float64     `json:"distance_km"`
	Energy      money.Cents `json:"energy"`
	Tolls       money.Cents `json:"tolls"`
	Maintenance money.Cents `json:"maintenance"`
	Insurance   money.Cents `json:"insurance"`
	Financing   money.Cents `json:"financing"`
	Other       money.Cents `json:"other"` // Subscriptions, taxes, accessories, other
	Tires       money.Cents `json:"tires"`
	Total       money.Cents `json:"total"`
	CostPerKm   float64     `json:"cost_per_km"`
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
	InsuranceMissing    bool     `json:"insurance_missing"`
	AcquisitionMissing  bool     `json:"acquisition_missing"`
	Warnings            []string `json:"warnings"`
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
	DistanceBasisKm    float64     `json:"distance_basis_km"`
	TotalCost          money.Cents `json:"total_cost"`
	TotalCostPerKm     float64     `json:"total_cost_per_km"`
	UsageCostPerKm     float64     `json:"usage_cost_per_km"`
	FullCost           money.Cents `json:"full_cost"`
	FullCostPerKm      float64     `json:"full_cost_per_km"`

	AcquisitionType       string      `json:"acquisition_type"`
	AcquisitionCost       money.Cents `json:"acquisition_cost"` // Purchase price net of incentives
	DepreciationCost      money.Cents `json:"depreciation_cost"`
	DepreciationCostPerKm float64     `json:"depreciation_cost_per_km"`

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

	InsuranceCost      money.Cents `json:"insurance_cost"`
	InsuranceCostPerKm float64     `json:"insurance_cost_per_km"`
	InsuranceSource    string      `json:"insurance_source"`

	FinancingCost      money.Cents `json:"financing_cost"` // Lease rents, loan interest
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
	timezone string
}

// NewTCOService creates a new TCOService; timezone (IANA name) is used for monthly buckets.
func NewTCOService(pool *pgxpool.Pool, timezone string) *TCOService {
	if timezone == "" {
		timezone = "UTC"
	}
	return &TCOService{pool: pool, timezone: timezone}
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
func round1(v float64) float64 { return math.Round(v*10) / 10 }

func perKm(amount money.Cents, km float64) float64 {
	if km <= 0 {
		return 0
	}
	return round3(amount.Float() / km)
}

// vehicleAcquisition is the acquisition configuration used for depreciation.
type vehicleAcquisition struct {
	acquisitionType  *string
	purchasePrice    *money.Cents
	purchaseDate     *time.Time
	purchaseOdometer *float64
	incentives       *money.Cents
	resaleValue      *money.Cents
	holdingMonths    *int
	currentOdometer  float64
	annualInsurance  *money.Cents
}

// Depreciation spreads the purchase price net of incentives and expected resale value linearly
// over the expected holding period. ok is false when the settings needed are missing.
func (a vehicleAcquisition) Depreciation(now time.Time) (amount money.Cents, ok bool) {
	if a.purchasePrice == nil || a.purchaseDate == nil || a.resaleValue == nil || a.holdingMonths == nil || *a.holdingMonths <= 0 {
		return 0, false
	}
	depreciable := *a.purchasePrice - *a.resaleValue
	if a.incentives != nil {
		depreciable -= *a.incentives
	}
	if depreciable <= 0 {
		return 0, true
	}
	elapsedMonths := now.Sub(*a.purchaseDate).Hours() / 24 / (365.25 / 12)
	ratio := math.Max(0, math.Min(1, elapsedMonths/float64(*a.holdingMonths)))
	return money.FromFloat(depreciable.Float() * ratio), true
}

// ComputeVehicleTCO calculates complete TCO for a given vehicle.
func (s *TCOService) ComputeVehicleTCO(ctx context.Context, vehicleID string) (*TCOSummary, error) {
	sum := &TCOSummary{}
	comp := &sum.Completeness
	now := time.Now()

	// 1. Vehicle acquisition settings
	var acq vehicleAcquisition
	if err := s.pool.QueryRow(ctx, `
		SELECT acquisition_type, purchase_price, purchase_date, purchase_odometer, purchase_incentives,
		       expected_resale_value, expected_holding_months, current_odometer, annual_insurance_cost
		FROM vehicles WHERE id = $1;
	`, vehicleID).Scan(&acq.acquisitionType, &acq.purchasePrice, &acq.purchaseDate, &acq.purchaseOdometer, &acq.incentives,
		&acq.resaleValue, &acq.holdingMonths, &acq.currentOdometer, &acq.annualInsurance); err != nil {
		return nil, fmt.Errorf("vehicle: %w", err)
	}

	// 2. Distance: tracked drives, odometer span, distance since acquisition
	var trackedKm, odometerSpan float64
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(distance_km), 0),
		       COALESCE(MAX(end_odometer) FILTER (WHERE end_odometer > 0) - MIN(start_odometer) FILTER (WHERE start_odometer > 0), 0)
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL;
	`, vehicleID).Scan(&trackedKm, &odometerSpan); err != nil {
		return nil, fmt.Errorf("distance: %w", err)
	}
	basisKm := math.Max(trackedKm, odometerSpan)
	if acq.purchaseOdometer != nil && acq.currentOdometer > *acq.purchaseOdometer {
		basisKm = math.Max(basisKm, acq.currentOdometer-*acq.purchaseOdometer)
	}
	if untracked := basisKm - trackedKm; untracked > 50 && untracked > 0.01*basisKm {
		comp.UntrackedDistanceKm = round1(untracked)
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%.0f km parcourus n'apparaissent dans aucun trajet (avant TeslaMate ou TeslaMate hors ligne) : le coût au km utilise la distance odométrique", untracked))
	}

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
	case acq.annualInsurance != nil && *acq.annualInsurance > 0:
		sum.InsuranceSource = InsuranceSourceVehicleSettings
	default:
		sum.InsuranceSource = InsuranceSourceNone
		comp.InsuranceMissing = true
		comp.Warnings = append(comp.Warnings, "Aucune assurance enregistrée (dépense « Assurance » ou montant annuel dans la fiche véhicule)")
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

	// 7. Acquisition and depreciation
	switch {
	case acq.acquisitionType == nil:
		comp.AcquisitionMissing = true
		comp.Warnings = append(comp.Warnings, "Mode d'acquisition non renseigné (achat ou location) : décote ou loyers absents du coût complet")
	case *acq.acquisitionType == "PURCHASE":
		sum.AcquisitionType = "PURCHASE"
		sum.AcquisitionCost = byCategory[LedgerAcquisition]
		dep, ok := acq.Depreciation(now)
		if !ok {
			comp.AcquisitionMissing = true
			comp.Warnings = append(comp.Warnings, "Valeur de revente estimée ou durée de détention non renseignée : décote exclue du coût complet")
		}
		sum.DepreciationCost = dep
	case *acq.acquisitionType == "LEASE":
		sum.AcquisitionType = "LEASE"
		if byCategory[LedgerFinancing] == 0 {
			comp.AcquisitionMissing = true
			comp.Warnings = append(comp.Warnings, "Véhicule en location sans loyer enregistré (dépense récurrente « Financement »)")
		}
	}

	// 8. Toll qualification backlog
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND `+database.UnqualifiedDrivePredicate+`;
	`, vehicleID).Scan(&comp.UnqualifiedDrives); err != nil {
		return nil, fmt.Errorf("unqualified drives: %w", err)
	}
	if comp.UnqualifiedDrives > 0 {
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%d trajet(s) de type autoroutier sans péage renseigné ni qualification « sans péage »", comp.UnqualifiedDrives))
	}
	if basisKm <= 0 {
		comp.Warnings = append(comp.Warnings, "Aucun kilométrage enregistré : le coût au kilomètre ne peut pas être calculé")
	}
	comp.IsComplete = len(comp.Warnings) == 0
	if comp.Warnings == nil {
		comp.Warnings = []string{}
	}

	// 9. Aggregates
	energy := byCategory[LedgerEnergy]
	travel := byCategory[LedgerToll] + byCategory[LedgerParking] + byCategory[LedgerTravelOther]
	tires := byCategory[LedgerTires]
	maintenance := byCategory[LedgerMaintenance]
	insurance := byCategory[LedgerInsurance]
	financing := byCategory[LedgerFinancing]
	subscription, tax := byCategory[LedgerSubscr], byCategory[LedgerTax]
	var other money.Cents
	for category, amount := range byCategory {
		switch category {
		case LedgerEnergy, LedgerToll, LedgerParking, LedgerTravelOther, LedgerTires, LedgerMaintenance,
			LedgerInsurance, LedgerFinancing, LedgerTax, LedgerSubscr, LedgerAcquisition:
		default:
			other += amount
		}
	}
	otherTotal := subscription + tax + other

	running := energy + travel + maintenance + insurance + financing + otherTotal
	sum.TotalCost = running + tires
	sum.FullCost = running + sum.TiresAmortizedCost + sum.DepreciationCost
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
	sum.InsuranceCost = insurance
	sum.InsuranceCostPerKm = perKm(insurance, basisKm)
	sum.FinancingCost = financing
	sum.FinancingCostPerKm = perKm(financing, basisKm)
	sum.SubscriptionCost = subscription
	sum.TaxCost = tax
	sum.OtherCost = other
	sum.OtherCostPerKm = perKm(otherTotal, basisKm)

	if sum.TagBreakdown, err = s.tagBreakdown(ctx, vehicleID, trackedKm); err != nil {
		return nil, fmt.Errorf("tag breakdown: %w", err)
	}
	if sum.MonthlyCosts, err = s.monthlyCosts(ctx, vehicleID, now); err != nil {
		return nil, fmt.Errorf("monthly costs: %w", err)
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

// monthlyCosts builds the cash-basis monthly timeline (acquisition excluded) in the reporting timezone.
func (s *TCOService) monthlyCosts(ctx context.Context, vehicleID string, now time.Time) ([]MonthlyCost, error) {
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
		return nil, err
	}
	for rows.Next() {
		var m, category string
		var amount money.Cents
		if err := rows.Scan(&m, &category, &amount); err != nil {
			rows.Close()
			return nil, err
		}
		mc := get(m)
		switch category {
		case LedgerEnergy:
			mc.Energy += amount
		case LedgerToll, LedgerParking, LedgerTravelOther:
			mc.Tolls += amount
		case LedgerTires:
			mc.Tires += amount
		case LedgerMaintenance:
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
		return nil, err
	}

	distRows, err := s.pool.Query(ctx, `
		SELECT TO_CHAR(start_time AT TIME ZONE $2, 'YYYY-MM') AS m, COALESCE(SUM(distance_km), 0)
		FROM drives WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL GROUP BY m;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, err
	}
	for distRows.Next() {
		var m string
		var km float64
		if err := distRows.Scan(&m, &km); err != nil {
			distRows.Close()
			return nil, err
		}
		get(m).DistanceKm += km
	}
	distRows.Close()
	if err := distRows.Err(); err != nil {
		return nil, err
	}

	monthlyCosts := make([]MonthlyCost, 0, len(monthlyMap))
	for _, mc := range monthlyMap {
		mc.DistanceKm = round1(mc.DistanceKm)
		mc.Total = mc.Energy + mc.Tolls + mc.Maintenance + mc.Insurance + mc.Financing + mc.Other + mc.Tires
		mc.CostPerKm = perKm(mc.Total, mc.DistanceKm)
		monthlyCosts = append(monthlyCosts, *mc)
	}
	sort.Slice(monthlyCosts, func(i, j int) bool {
		return monthlyCosts[i].Month < monthlyCosts[j].Month
	})

	if len(monthlyCosts) == 0 {
		monthlyCosts = append(monthlyCosts, MonthlyCost{Month: now.Format("2006-01")})
	}
	return monthlyCosts, nil
}
