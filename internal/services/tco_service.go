package services

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/database"
)

// maintenanceOccurrencesCTE expands maintenance expenses of vehicle $1 into dated occurrences (EUR):
// a recurring expense produces one occurrence per interval until today or its recurrence end date.
const maintenanceOccurrencesCTE = `
	maintenance_occurrences AS (
		SELECT m.id, m.category, m.currency, m.fx_rate, occ AS occ_date,
		       (CASE WHEN m.currency = 'EUR' THEN m.amount ELSE m.amount * m.fx_rate END) AS amount_eur
		FROM maintenance_expenses m
		CROSS JOIN LATERAL generate_series(
			m.date,
			CASE
				WHEN m.is_recurring AND COALESCE(m.recurrence_interval_months, 0) > 0
				THEN GREATEST(m.date, LEAST(NOW(), COALESCE(m.recurrence_end_date, NOW())))
				ELSE m.date
			END,
			make_interval(months => GREATEST(COALESCE(m.recurrence_interval_months, 1), 1))
		) AS occ
		WHERE m.vehicle_id = $1
	)
`

// Cost category buckets of maintenance_expenses.
const (
	categoryMaintenance = "MAINTENANCE"
	categoryInsurance   = "INSURANCE"
)

// Insurance sources.
const (
	InsuranceSourceRecordedExpenses = "RECORDED_EXPENSES"
	InsuranceSourceVehicleSettings  = "VEHICLE_SETTINGS"
	InsuranceSourceNone             = "NONE"
	InsuranceSourceDefault          = "DEFAULT"
)

// MonthlyCost represents monthly expenditure (cash basis) and mileage breakdown.
type MonthlyCost struct {
	Month       string  `json:"month"` // YYYY-MM
	DistanceKm  float64 `json:"distance_km"`
	Energy      float64 `json:"energy"`
	Tolls       float64 `json:"tolls"`
	Maintenance float64 `json:"maintenance"`
	Insurance   float64 `json:"insurance"`
	Other       float64 `json:"other"` // Subscriptions, taxes, accessories, other
	Tires       float64 `json:"tires"`
	Total       float64 `json:"total"`
	CostPerKm   float64 `json:"cost_per_km"`
}

// TagCostBreakdown represents costs split by tag (e.g. Pro vs Perso).
type TagCostBreakdown struct {
	Tag         string  `json:"tag"`
	DistanceKm  float64 `json:"distance_km"`
	EnergyKwh   float64 `json:"energy_kwh"`
	TollsAmount float64 `json:"tolls_amount"`
	Percentage  float64 `json:"percentage"`
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
	Warnings            []string `json:"warnings"`
}

// TCOSummary represents the global TCO calculation.
//
// Cash figures (TotalCost, *Cost) are what was actually paid. Per-km figures use DistanceBasisKm,
// the largest of the distance tracked by drives and the odometer span covered by those drives.
// UsageCostPerKm is the marginal cost of driving (energy + tolls/parking); FullCostPerKm adds
// amortized tires and all running costs.
type TCOSummary struct {
	TotalDistanceKm    float64 `json:"total_distance_km"`
	OdometerDistanceKm float64 `json:"odometer_distance_km"`
	DistanceBasisKm    float64 `json:"distance_basis_km"`
	TotalCost          float64 `json:"total_cost"`
	TotalCostPerKm     float64 `json:"total_cost_per_km"`
	UsageCostPerKm     float64 `json:"usage_cost_per_km"`
	FullCost           float64 `json:"full_cost"`
	FullCostPerKm      float64 `json:"full_cost_per_km"`

	// Category breakdowns
	EnergyCost      float64 `json:"energy_cost"`
	EnergyCostPerKm float64 `json:"energy_cost_per_km"`
	TotalKwhAdded   float64 `json:"total_kwh_added"`
	AvgCostPerKwh   float64 `json:"avg_cost_per_kwh"`

	TollsCost      float64 `json:"tolls_cost"`
	TollsCostPerKm float64 `json:"tolls_cost_per_km"`

	TiresCost               float64 `json:"tires_cost"`
	TiresCostPerKm          float64 `json:"tires_cost_per_km"`
	TiresAmortizedCost      float64 `json:"tires_amortized_cost"`
	TiresAmortizedCostPerKm float64 `json:"tires_amortized_cost_per_km"`

	MaintenanceCost      float64 `json:"maintenance_cost"`
	MaintenanceCostPerKm float64 `json:"maintenance_cost_per_km"`

	InsuranceCost      float64 `json:"insurance_cost"`
	InsuranceCostPerKm float64 `json:"insurance_cost_per_km"`
	InsuranceSource    string  `json:"insurance_source"`

	SubscriptionCost float64 `json:"subscription_cost"`
	TaxCost          float64 `json:"tax_cost"`
	OtherCost        float64 `json:"other_cost"`
	OtherCostPerKm   float64 `json:"other_cost_per_km"` // Subscriptions + taxes + other

	// Tag analysis (Pro / Perso)
	TagBreakdown []TagCostBreakdown `json:"tag_breakdown"`

	// Timeline
	MonthlyCosts []MonthlyCost `json:"monthly_costs"`

	Completeness TCOCompleteness `json:"completeness"`
}

// TCOService computes TCO metrics using optimized SQL queries.
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

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
func round1(v float64) float64 { return math.Round(v*10) / 10 }

func perKm(amount, km float64) float64 {
	if km <= 0 {
		return 0
	}
	return round3(amount / km)
}

// ComputeVehicleTCO calculates complete TCO for a given vehicle.
func (s *TCOService) ComputeVehicleTCO(ctx context.Context, vehicleID string) (*TCOSummary, error) {
	sum := &TCOSummary{}
	comp := &sum.Completeness

	// 1. Distance: tracked drives vs odometer span
	var trackedKm, odometerSpan float64
	var firstDrive *time.Time
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(distance_km), 0),
		       COALESCE(MAX(end_odometer) FILTER (WHERE end_odometer > 0) - MIN(start_odometer) FILTER (WHERE start_odometer > 0), 0),
		       MIN(start_time)
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL;
	`, vehicleID).Scan(&trackedKm, &odometerSpan, &firstDrive); err != nil {
		return nil, fmt.Errorf("distance: %w", err)
	}
	basisKm := math.Max(trackedKm, odometerSpan)
	if untracked := odometerSpan - trackedKm; untracked > 50 && untracked > 0.01*odometerSpan {
		comp.UntrackedDistanceKm = round1(untracked)
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%.0f km couverts par l'odomètre n'apparaissent dans aucun trajet (TeslaMate hors ligne ?) : le coût au km utilise la distance odométrique", untracked))
	}

	// 2. Energy
	var energyCost, kwhAdded, kwhPriced float64
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(CASE WHEN currency = 'EUR' THEN cost ELSE cost * fx_rate END), 0),
		       COALESCE(SUM(kwh_added), 0),
		       COALESCE(SUM(kwh_added) FILTER (WHERE cost IS NOT NULL AND (currency = 'EUR' OR fx_rate IS NOT NULL)), 0),
		       COUNT(*) FILTER (WHERE cost IS NULL),
		       COALESCE(SUM(kwh_added) FILTER (WHERE cost IS NULL), 0),
		       COUNT(*) FILTER (WHERE cost IS NOT NULL AND currency <> 'EUR' AND fx_rate IS NULL)
		FROM charge_logs
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL;
	`, vehicleID).Scan(&energyCost, &kwhAdded, &kwhPriced, &comp.ChargesWithoutCost, &comp.KwhWithoutCost, &comp.UnconvertedExpenses); err != nil {
		return nil, fmt.Errorf("energy: %w", err)
	}
	if comp.ChargesWithoutCost > 0 {
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%d recharge(s) sans coût (%.0f kWh) : coût énergétique sous-estimé", comp.ChargesWithoutCost, comp.KwhWithoutCost))
	}

	// 3. Drive expenses (tolls, parking, ferry...)
	var tollsCost float64
	var unconvertedTolls int
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(`+database.AmountEURExpr+`), 0),
		       COUNT(*) FILTER (WHERE currency <> 'EUR' AND fx_rate IS NULL)
		FROM drive_expenses
		WHERE vehicle_id = $1;
	`, vehicleID).Scan(&tollsCost, &unconvertedTolls); err != nil {
		return nil, fmt.Errorf("drive expenses: %w", err)
	}

	// 4. Tires: cash purchases and amortized consumption
	var tiresCash, tiresAmortized float64
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(t.purchase_price), 0),
		       COALESCE(SUM(
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
	`, vehicleID).Scan(&tiresCash, &tiresAmortized); err != nil {
		return nil, fmt.Errorf("tires: %w", err)
	}

	// 5. Maintenance, insurance, subscriptions, taxes (recurring expenses expanded)
	var maintenance, insurance, subscription, tax, other float64
	var unconvertedMaint, insuranceEntries int
	if err := s.pool.QueryRow(ctx, `
		WITH `+maintenanceOccurrencesCTE+`
		SELECT COALESCE(SUM(amount_eur) FILTER (WHERE category = 'MAINTENANCE'), 0),
		       COALESCE(SUM(amount_eur) FILTER (WHERE category = 'INSURANCE'), 0),
		       COALESCE(SUM(amount_eur) FILTER (WHERE category = 'SUBSCRIPTION'), 0),
		       COALESCE(SUM(amount_eur) FILTER (WHERE category = 'TAX'), 0),
		       COALESCE(SUM(amount_eur) FILTER (WHERE category NOT IN ('MAINTENANCE', 'INSURANCE', 'SUBSCRIPTION', 'TAX')), 0),
		       COUNT(DISTINCT id) FILTER (WHERE currency <> 'EUR' AND fx_rate IS NULL),
		       COUNT(*) FILTER (WHERE category = 'INSURANCE')
		FROM maintenance_occurrences;
	`, vehicleID).Scan(&maintenance, &insurance, &subscription, &tax, &other, &unconvertedMaint, &insuranceEntries); err != nil {
		return nil, fmt.Errorf("maintenance: %w", err)
	}
	comp.UnconvertedExpenses += unconvertedTolls + unconvertedMaint
	if comp.UnconvertedExpenses > 0 {
		comp.Warnings = append(comp.Warnings, fmt.Sprintf(
			"%d dépense(s) en devise étrangère sans taux de conversion : exclues des totaux", comp.UnconvertedExpenses))
	}

	// 6. Insurance from vehicle settings when no insurance expense is recorded (pro rata since first activity)
	var annualInsurance *float64
	var vehicleCreated time.Time
	if err := s.pool.QueryRow(ctx, `SELECT annual_insurance_cost, created_at FROM vehicles WHERE id = $1;`, vehicleID).
		Scan(&annualInsurance, &vehicleCreated); err != nil {
		return nil, fmt.Errorf("vehicle: %w", err)
	}
	loc := s.location()
	now := time.Now().In(loc)
	insuranceByMonth := map[string]float64{}
	switch {
	case insuranceEntries > 0:
		sum.InsuranceSource = InsuranceSourceRecordedExpenses
	case annualInsurance != nil && *annualInsurance > 0:
		sum.InsuranceSource = InsuranceSourceVehicleSettings
		start := vehicleCreated
		if firstDrive != nil && firstDrive.Before(start) {
			start = *firstDrive
		}
		insuranceByMonth = prorateAnnualAmount(*annualInsurance, start.In(loc), now)
		for _, v := range insuranceByMonth {
			insurance += v
		}
	default:
		sum.InsuranceSource = InsuranceSourceNone
		comp.InsuranceMissing = true
		comp.Warnings = append(comp.Warnings, "Aucune assurance enregistrée (dépense « Assurance » ou montant annuel dans la fiche véhicule)")
	}

	// 7. Toll qualification backlog
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

	otherTotal := subscription + tax + other
	totalCash := energyCost + tollsCost + tiresCash + maintenance + insurance + otherTotal
	fullCost := energyCost + tollsCost + tiresAmortized + maintenance + insurance + otherTotal

	sum.TotalDistanceKm = round1(trackedKm)
	sum.OdometerDistanceKm = round1(odometerSpan)
	sum.DistanceBasisKm = round1(basisKm)
	sum.TotalCost = round2(totalCash)
	sum.TotalCostPerKm = perKm(totalCash, basisKm)
	sum.UsageCostPerKm = perKm(energyCost+tollsCost, basisKm)
	sum.FullCost = round2(fullCost)
	sum.FullCostPerKm = perKm(fullCost, basisKm)
	sum.EnergyCost = round2(energyCost)
	sum.EnergyCostPerKm = perKm(energyCost, basisKm)
	sum.TotalKwhAdded = round1(kwhAdded)
	if kwhPriced > 0 {
		sum.AvgCostPerKwh = round3(energyCost / kwhPriced)
	}
	sum.TollsCost = round2(tollsCost)
	sum.TollsCostPerKm = perKm(tollsCost, basisKm)
	sum.TiresCost = round2(tiresCash)
	sum.TiresCostPerKm = perKm(tiresCash, basisKm)
	sum.TiresAmortizedCost = round2(tiresAmortized)
	sum.TiresAmortizedCostPerKm = perKm(tiresAmortized, basisKm)
	sum.MaintenanceCost = round2(maintenance)
	sum.MaintenanceCostPerKm = perKm(maintenance, basisKm)
	sum.InsuranceCost = round2(insurance)
	sum.InsuranceCostPerKm = perKm(insurance, basisKm)
	sum.SubscriptionCost = round2(subscription)
	sum.TaxCost = round2(tax)
	sum.OtherCost = round2(other)
	sum.OtherCostPerKm = perKm(otherTotal, basisKm)

	var err error
	if sum.TagBreakdown, err = s.tagBreakdown(ctx, vehicleID, trackedKm); err != nil {
		return nil, fmt.Errorf("tag breakdown: %w", err)
	}
	if sum.MonthlyCosts, err = s.monthlyCosts(ctx, vehicleID, insuranceByMonth, now); err != nil {
		return nil, fmt.Errorf("monthly costs: %w", err)
	}
	if comp.Warnings == nil {
		comp.Warnings = []string{}
	}
	return sum, nil
}

func (s *TCOService) location() *time.Location {
	if loc, err := time.LoadLocation(s.timezone); err == nil {
		return loc
	}
	return time.UTC
}

// prorateAnnualAmount spreads an annual amount day by day between start and end, grouped by month (YYYY-MM).
func prorateAnnualAmount(annual float64, start, end time.Time) map[string]float64 {
	out := map[string]float64{}
	if !end.After(start) || annual <= 0 {
		return out
	}
	daily := annual / 365.25
	cursor := start
	for cursor.Before(end) {
		monthStart := time.Date(cursor.Year(), cursor.Month(), 1, 0, 0, 0, 0, cursor.Location())
		next := monthStart.AddDate(0, 1, 0)
		if next.After(end) {
			next = end
		}
		out[cursor.Format("2006-01")] += daily * next.Sub(cursor).Hours() / 24
		cursor = next
	}
	return out
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
		tb.TollsAmount = round2(tb.TollsAmount)
		list = append(list, tb)
	}
	return list, rows.Err()
}

// monthlyCosts builds the cash-basis monthly timeline in the reporting timezone.
func (s *TCOService) monthlyCosts(ctx context.Context, vehicleID string, insuranceByMonth map[string]float64, now time.Time) ([]MonthlyCost, error) {
	monthlyMap := make(map[string]*MonthlyCost)
	get := func(m string) *MonthlyCost {
		if _, ok := monthlyMap[m]; !ok {
			monthlyMap[m] = &MonthlyCost{Month: m}
		}
		return monthlyMap[m]
	}

	type series struct {
		query string
		apply func(mc *MonthlyCost, val float64)
	}
	all := []series{
		{`SELECT TO_CHAR(date AT TIME ZONE $2, 'YYYY-MM') AS m,
		         COALESCE(SUM(CASE WHEN currency = 'EUR' THEN cost ELSE cost * fx_rate END), 0)
		  FROM charge_logs WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL GROUP BY m`,
			func(mc *MonthlyCost, v float64) { mc.Energy += v }},
		{`SELECT TO_CHAR(date AT TIME ZONE $2, 'YYYY-MM') AS m, COALESCE(SUM(` + database.AmountEURExpr + `), 0)
		  FROM drive_expenses WHERE vehicle_id = $1 GROUP BY m`,
			func(mc *MonthlyCost, v float64) { mc.Tolls += v }},
		{`SELECT TO_CHAR(purchase_date, 'YYYY-MM') AS m, COALESCE(SUM(purchase_price), 0)
		  FROM tires WHERE vehicle_id = $1 AND $2::text IS NOT NULL GROUP BY m`,
			func(mc *MonthlyCost, v float64) { mc.Tires += v }},
		{`SELECT TO_CHAR(start_time AT TIME ZONE $2, 'YYYY-MM') AS m, COALESCE(SUM(distance_km), 0)
		  FROM drives WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL GROUP BY m`,
			func(mc *MonthlyCost, v float64) { mc.DistanceKm += v }},
	}
	for _, sr := range all {
		if err := s.scanMonthly(ctx, sr.query, vehicleID, func(m string, v float64) { sr.apply(get(m), v) }); err != nil {
			return nil, err
		}
	}

	rows, err := s.pool.Query(ctx, `
		WITH `+maintenanceOccurrencesCTE+`
		SELECT TO_CHAR(occ_date AT TIME ZONE $2, 'YYYY-MM') AS m, category, COALESCE(SUM(amount_eur), 0)
		FROM maintenance_occurrences
		GROUP BY m, category;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var m, category string
		var val float64
		if err := rows.Scan(&m, &category, &val); err != nil {
			rows.Close()
			return nil, err
		}
		mc := get(m)
		switch category {
		case categoryMaintenance:
			mc.Maintenance += val
		case categoryInsurance:
			mc.Insurance += val
		default:
			mc.Other += val
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for m, v := range insuranceByMonth {
		get(m).Insurance += v
	}

	monthlyCosts := make([]MonthlyCost, 0, len(monthlyMap))
	for _, mc := range monthlyMap {
		mc.Energy = round2(mc.Energy)
		mc.Tolls = round2(mc.Tolls)
		mc.Tires = round2(mc.Tires)
		mc.Maintenance = round2(mc.Maintenance)
		mc.Insurance = round2(mc.Insurance)
		mc.Other = round2(mc.Other)
		mc.DistanceKm = round1(mc.DistanceKm)
		mc.Total = round2(mc.Energy + mc.Tolls + mc.Maintenance + mc.Insurance + mc.Other + mc.Tires)
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

func (s *TCOService) scanMonthly(ctx context.Context, query, vehicleID string, apply func(month string, val float64)) error {
	rows, err := s.pool.Query(ctx, query, vehicleID, s.timezone)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var m string
		var val float64
		if err := rows.Scan(&m, &val); err != nil {
			return err
		}
		apply(m, val)
	}
	return rows.Err()
}
