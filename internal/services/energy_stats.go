package services

import (
	"context"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/money"
)

// Charging sessions are bucketed by their average power (energy added over the session duration). The average
// includes the taper and the ramp-up, so the thresholds sit well below the nominal power of the equipment.
const (
	// A domestic socket delivers 2.3 kW (10 A) to 3.7 kW (16 A).
	slowChargeMaxKw = 3.5
	// Wallboxes and public AC posts stay at or below 22 kW; DC sessions average far above that.
	acChargeMaxKw = 25.0
)

// Charge classes reported by the energy statistics.
const (
	ChargeClassSlow    = "SLOW"    // Domestic socket
	ChargeClassAC      = "AC"      // Wallbox or public AC post
	ChargeClassDC      = "DC"      // Fast charging
	ChargeClassUnknown = "UNKNOWN" // No usable duration (manual entries)
)

const (
	// A month or window with less distance than this gives a cost per 100 km dominated by charging timing.
	minDistanceForCostPer100km = 50.0
	minDistanceForTrailing     = 100.0
	trailingMonths             = 3

	// The energy drawn from the grid always exceeds the energy stored, so a genuine ratio is below 1. TeslaMateApi
	// reports GREATEST(energy used, energy added): when the grid energy was not measured (typically DC charging)
	// both are equal and the ratio is exactly 1, which means "unknown" rather than a lossless charge. Ratios
	// under the minimum come from truncated readings.
	minPlausibleChargeEfficiency = 0.6
	maxPlausibleChargeEfficiency = 1.0 // exclusive
)

// EnergyMonth is the energy activity of one calendar month.
type EnergyMonth struct {
	Month               string      `json:"month"` // YYYY-MM
	DistanceKm          float64     `json:"distance_km"`
	DriveKwh            float64     `json:"drive_kwh"`
	ConsumptionKwh100km *float64    `json:"consumption_kwh_100km,omitempty"`
	KwhAdded            float64     `json:"kwh_added"`
	ChargeEfficiency    *float64    `json:"charge_efficiency,omitempty"`
	EnergyCost          money.Cents `json:"energy_cost"`
	PricePerKwh         *float64    `json:"price_per_kwh,omitempty"`
	CostPer100km        *float64    `json:"cost_per_100km,omitempty"`
	// CostPer100kmTrailing smooths the gap between when energy is bought and when it is driven.
	CostPer100kmTrailing *float64 `json:"cost_per_100km_trailing,omitempty"`
}

// ChargeClassStat aggregates the charging sessions of one class over the whole history.
type ChargeClassStat struct {
	Class            string      `json:"class"`
	Sessions         int         `json:"sessions"`
	KwhAdded         float64     `json:"kwh_added"`
	EnergyCost       money.Cents `json:"energy_cost"`
	PricePerKwh      *float64    `json:"price_per_kwh,omitempty"`
	ChargeEfficiency *float64    `json:"charge_efficiency,omitempty"`
}

// EnergySummary holds the figures over the whole history.
type EnergySummary struct {
	DistanceKm          float64  `json:"distance_km"`
	KwhAdded            float64  `json:"kwh_added"`
	ConsumptionKwh100km *float64 `json:"consumption_kwh_100km,omitempty"`
	ChargeEfficiency    *float64 `json:"charge_efficiency,omitempty"`
	PricePerKwh         *float64 `json:"price_per_kwh,omitempty"`
	CostPer100km        *float64 `json:"cost_per_100km,omitempty"`
	SessionsWithoutCost int      `json:"sessions_without_cost"`
}

// EnergyStats is the cost-aware efficiency view of a vehicle: what it consumes, what the energy costs and how
// it was bought.
type EnergyStats struct {
	Months        []EnergyMonth     `json:"months"`
	ChargeClasses []ChargeClassStat `json:"charge_classes"`
	Summary       EnergySummary     `json:"summary"`
}

// energyDriveMonth is the drives of a month, aggregated by the database.
type energyDriveMonth struct {
	Month      string
	DistanceKm float64
	// MeasuredKm is the distance of the drives that report their energy consumption.
	MeasuredKm float64
	Kwh        float64
}

// energyCharge is one charging session.
type energyCharge struct {
	Month    string
	Start    time.Time
	End      *time.Time
	KwhAdded float64
	KwhUsed  *float64 // Energy drawn from the grid
	Cost     *money.Cents
}

// classifyCharge buckets a session by average power. ok is false when no power can be derived.
func classifyCharge(kwhAdded float64, start time.Time, end *time.Time) (class string, ok bool) {
	if end == nil || kwhAdded <= 0 {
		return ChargeClassUnknown, false
	}
	hours := end.Sub(start).Hours()
	if hours <= 0 {
		return ChargeClassUnknown, false
	}
	kw := kwhAdded / hours
	switch {
	case kw < slowChargeMaxKw:
		return ChargeClassSlow, true
	case kw < acChargeMaxKw:
		return ChargeClassAC, true
	default:
		return ChargeClassDC, true
	}
}

// efficiencyRatio is the energy stored over the energy drawn, ok only for a plausible reading.
func efficiencyRatio(added float64, used *float64) (ratio float64, ok bool) {
	if used == nil || *used <= 0 || added <= 0 {
		return 0, false
	}
	r := added / *used
	if r < minPlausibleChargeEfficiency || r >= maxPlausibleChargeEfficiency {
		return 0, false
	}
	return r, true
}

type energyAcc struct {
	distanceKm, measuredKm, driveKwh float64
	kwhAdded                         float64
	effAdded, effUsed                float64
	cost                             money.Cents
	kwhPriced                        float64
}

func (a *energyAcc) addCharge(c energyCharge) {
	a.kwhAdded += c.KwhAdded
	if c.Cost != nil {
		a.cost += *c.Cost
		a.kwhPriced += c.KwhAdded
	}
	if _, ok := efficiencyRatio(c.KwhAdded, c.KwhUsed); ok {
		a.effAdded += c.KwhAdded
		a.effUsed += *c.KwhUsed
	}
}

func ratioPtr(num, den float64, digits int) *float64 {
	if den <= 0 || num <= 0 {
		return nil
	}
	f := 1.0
	for i := 0; i < digits; i++ {
		f *= 10
	}
	v := float64(int64(num/den*f+0.5)) / f
	return &v
}

// previousMonths lists the n calendar months ending at month (inclusive), oldest first.
func previousMonths(month string, n int) []string {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return []string{month}
	}
	out := make([]string, 0, n)
	for i := n - 1; i >= 0; i-- {
		out = append(out, t.AddDate(0, -i, 0).Format("2006-01"))
	}
	return out
}

// computeEnergyStats derives the statistics from drives aggregated per month and from charging sessions.
func computeEnergyStats(drives []energyDriveMonth, charges []energyCharge) *EnergyStats {
	months := map[string]*energyAcc{}
	get := func(m string) *energyAcc {
		if months[m] == nil {
			months[m] = &energyAcc{}
		}
		return months[m]
	}

	total := energyAcc{}
	for _, d := range drives {
		a := get(d.Month)
		a.distanceKm += d.DistanceKm
		a.measuredKm += d.MeasuredKm
		a.driveKwh += d.Kwh
		total.distanceKm += d.DistanceKm
		total.measuredKm += d.MeasuredKm
		total.driveKwh += d.Kwh
	}

	type classAcc struct {
		energyAcc
		sessions int
	}
	classes := map[string]*classAcc{}
	sessionsWithoutCost := 0
	for _, c := range charges {
		get(c.Month).addCharge(c)
		total.addCharge(c)
		if c.Cost == nil {
			sessionsWithoutCost++
		}
		class, _ := classifyCharge(c.KwhAdded, c.Start, c.End)
		if classes[class] == nil {
			classes[class] = &classAcc{}
		}
		classes[class].sessions++
		classes[class].addCharge(c)
	}

	keys := make([]string, 0, len(months))
	for m := range months {
		keys = append(keys, m)
	}
	sort.Strings(keys)

	out := &EnergyStats{Months: make([]EnergyMonth, 0, len(keys)), ChargeClasses: []ChargeClassStat{}}
	for _, m := range keys {
		a := months[m]
		em := EnergyMonth{
			Month:               m,
			DistanceKm:          round1(a.distanceKm),
			DriveKwh:            round1(a.driveKwh),
			ConsumptionKwh100km: ratioPtr(a.driveKwh*100, a.measuredKm, 1),
			KwhAdded:            round1(a.kwhAdded),
			ChargeEfficiency:    ratioPtr(a.effAdded, a.effUsed, 3),
			EnergyCost:          a.cost,
			PricePerKwh:         ratioPtr(a.cost.Float(), a.kwhPriced, 3),
		}
		if a.distanceKm >= minDistanceForCostPer100km {
			em.CostPer100km = ratioPtr(a.cost.Float()*100, a.distanceKm, 2)
		}

		// Trailing window: energy bought and energy driven rarely fall in the same month. Months without tracked
		// distance (before TeslaMate) are left out, their energy has no matching kilometres.
		var winKm float64
		var winCost money.Cents
		for _, wm := range previousMonths(m, trailingMonths) {
			if w := months[wm]; w != nil && w.distanceKm > 0 {
				winKm += w.distanceKm
				winCost += w.cost
			}
		}
		if winKm >= minDistanceForTrailing {
			em.CostPer100kmTrailing = ratioPtr(winCost.Float()*100, winKm, 2)
		}
		out.Months = append(out.Months, em)
	}

	for _, class := range []string{ChargeClassSlow, ChargeClassAC, ChargeClassDC, ChargeClassUnknown} {
		c := classes[class]
		if c == nil {
			continue
		}
		out.ChargeClasses = append(out.ChargeClasses, ChargeClassStat{
			Class:            class,
			Sessions:         c.sessions,
			KwhAdded:         round1(c.kwhAdded),
			EnergyCost:       c.cost,
			PricePerKwh:      ratioPtr(c.cost.Float(), c.kwhPriced, 3),
			ChargeEfficiency: ratioPtr(c.effAdded, c.effUsed, 3),
		})
	}

	// Cost per 100 km only counts the months where distance is tracked: energy bought before TeslaMate has no
	// matching distance and would inflate it.
	var trackedCost money.Cents
	var trackedKm float64
	for _, a := range months {
		if a.distanceKm > 0 {
			trackedCost += a.cost
			trackedKm += a.distanceKm
		}
	}
	out.Summary = EnergySummary{
		DistanceKm:          round1(total.distanceKm),
		KwhAdded:            round1(total.kwhAdded),
		ConsumptionKwh100km: ratioPtr(total.driveKwh*100, total.measuredKm, 1),
		ChargeEfficiency:    ratioPtr(total.effAdded, total.effUsed, 3),
		PricePerKwh:         ratioPtr(total.cost.Float(), total.kwhPriced, 3),
		SessionsWithoutCost: sessionsWithoutCost,
	}
	if trackedKm >= minDistanceForCostPer100km {
		out.Summary.CostPer100km = ratioPtr(trackedCost.Float()*100, trackedKm, 2)
	}
	return out
}

// EnergyStatsService computes the cost-aware efficiency statistics of a vehicle.
type EnergyStatsService struct {
	pool     *pgxpool.Pool
	timezone string
}

func NewEnergyStatsService(pool *pgxpool.Pool, timezone string) *EnergyStatsService {
	if timezone == "" {
		timezone = "UTC"
	}
	return &EnergyStatsService{pool: pool, timezone: timezone}
}

// Compute reads the drives and charges of a vehicle and derives its energy statistics.
func (s *EnergyStatsService) Compute(ctx context.Context, vehicleID string) (*EnergyStats, error) {
	driveRows, err := s.pool.Query(ctx, `
		SELECT TO_CHAR(start_time AT TIME ZONE $2, 'YYYY-MM') AS m,
		       COALESCE(SUM(distance_km), 0)::float8,
		       COALESCE(SUM(distance_km) FILTER (WHERE energy_consumed_kwh > 0), 0)::float8,
		       COALESCE(SUM(energy_consumed_kwh) FILTER (WHERE energy_consumed_kwh > 0), 0)::float8
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL
		GROUP BY m;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, err
	}
	var drives []energyDriveMonth
	for driveRows.Next() {
		var d energyDriveMonth
		if err := driveRows.Scan(&d.Month, &d.DistanceKm, &d.MeasuredKm, &d.Kwh); err != nil {
			driveRows.Close()
			return nil, err
		}
		drives = append(drives, d)
	}
	driveRows.Close()
	if err := driveRows.Err(); err != nil {
		return nil, err
	}

	// The cost is converted to EUR the same way as in the cost ledger; a foreign cost without rate stays unknown.
	chargeRows, err := s.pool.Query(ctx, `
		SELECT TO_CHAR(date AT TIME ZONE $2, 'YYYY-MM') AS m, date, end_date, kwh_added::float8, kwh_used::float8,
		       CASE WHEN cost IS NULL THEN NULL
		            WHEN currency = 'EUR' THEN ROUND(cost, 2)
		            WHEN fx_rate IS NOT NULL THEN ROUND(cost * fx_rate, 2) END AS cost_eur
		FROM charge_logs
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL
		ORDER BY date;
	`, vehicleID, s.timezone)
	if err != nil {
		return nil, err
	}
	var charges []energyCharge
	for chargeRows.Next() {
		var c energyCharge
		if err := chargeRows.Scan(&c.Month, &c.Start, &c.End, &c.KwhAdded, &c.KwhUsed, &c.Cost); err != nil {
			chargeRows.Close()
			return nil, err
		}
		charges = append(charges, c)
	}
	chargeRows.Close()
	if err := chargeRows.Err(); err != nil {
		return nil, err
	}

	return computeEnergyStats(drives, charges), nil
}
