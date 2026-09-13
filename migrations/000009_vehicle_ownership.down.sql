-- ============================================================================
-- TeslaCost Vehicle Ownership, Financing & Insurance Migration (Down)
-- ============================================================================

DROP VIEW IF EXISTS cost_ledger;

ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS annual_insurance_cost NUMERIC(10, 2);
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS annual_expected_mileage NUMERIC(10, 2) DEFAULT 15000;
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS acquisition_type VARCHAR(20);
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS purchase_price NUMERIC(10, 2);
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS purchase_date DATE;
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS purchase_odometer NUMERIC(10, 2);
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS purchase_incentives NUMERIC(10, 2);
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS expected_resale_value NUMERIC(10, 2);
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS expected_holding_months INT;
ALTER TABLE vehicles DROP CONSTRAINT IF EXISTS chk_vehicle_acquisition_type;
ALTER TABLE vehicles ADD CONSTRAINT chk_vehicle_acquisition_type
    CHECK (acquisition_type IS NULL OR acquisition_type IN ('PURCHASE', 'LEASE'));

-- Best-effort restoration: loan and lease details have no equivalent in the previous schema.
UPDATE vehicles v
SET acquisition_type = CASE WHEN o.acquisition_type IN ('CASH', 'LOAN') THEN 'PURCHASE' ELSE 'LEASE' END,
    purchase_price = o.purchase_price,
    purchase_date = o.start_date,
    purchase_odometer = o.start_odometer,
    purchase_incentives = o.incentives,
    expected_resale_value = o.expected_resale_value,
    expected_holding_months = o.expected_holding_months
FROM vehicle_ownership o
WHERE o.vehicle_id = v.id;

-- Insurance premiums created by the up migration are kept as recorded expenses.
DROP TABLE IF EXISTS vehicle_ownership;

CREATE OR REPLACE VIEW cost_ledger AS
-- Energy
SELECT c.vehicle_id,
       c.date AS entry_date,
       'ENERGY'::text AS category,
       'charge_logs'::text AS source_table,
       c.id AS source_id,
       ROUND(CASE WHEN c.currency = 'EUR' THEN c.cost ELSE c.cost * c.fx_rate END, 2) AS amount_eur
FROM charge_logs c
WHERE c.deleted_upstream_at IS NULL
  AND c.cost IS NOT NULL
  AND (c.currency = 'EUR' OR c.fx_rate IS NOT NULL)

UNION ALL
-- Travel expenses (tolls, parking, ferries)
SELECT e.vehicle_id,
       e.date,
       CASE e.type WHEN 'TOLL' THEN 'TOLL' WHEN 'PARKING' THEN 'PARKING' ELSE 'TRAVEL_OTHER' END,
       'drive_expenses',
       e.id,
       ROUND(CASE WHEN e.currency = 'EUR' THEN e.amount ELSE e.amount * e.fx_rate END, 2)
FROM drive_expenses e
WHERE e.currency = 'EUR' OR e.fx_rate IS NOT NULL

UNION ALL
-- Maintenance and fixed expenses, recurring ones expanded until today or their end date
SELECT m.vehicle_id,
       occ,
       m.category,
       'maintenance_expenses',
       m.id,
       ROUND(CASE WHEN m.currency = 'EUR' THEN m.amount ELSE m.amount * m.fx_rate END, 2)
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
WHERE m.currency = 'EUR' OR m.fx_rate IS NOT NULL

UNION ALL
-- Tire purchases
SELECT t.vehicle_id,
       t.purchase_date::timestamptz,
       'TIRES',
       'tires',
       t.id,
       t.purchase_price
FROM tires t
WHERE t.vehicle_id IS NOT NULL AND t.purchase_price > 0

UNION ALL
-- Annual insurance premium of the vehicle settings, pro rata per month, when no insurance expense is recorded
SELECT v.id,
       b.period_start,
       'INSURANCE',
       'vehicles',
       v.id,
       ROUND(v.annual_insurance_cost * EXTRACT(EPOCH FROM (b.period_end - b.period_start)) / (365.25 * 86400), 2)
FROM vehicles v
CROSS JOIN LATERAL (
    SELECT LEAST(
        v.created_at,
        COALESCE(v.purchase_date::timestamptz, v.created_at),
        COALESCE((SELECT MIN(d.start_time) FROM drives d WHERE d.vehicle_id = v.id AND d.deleted_upstream_at IS NULL), v.created_at)
    ) AS coverage_start
) s
CROSS JOIN LATERAL generate_series(date_trunc('month', s.coverage_start), NOW(), INTERVAL '1 month') AS month_start
CROSS JOIN LATERAL (
    SELECT GREATEST(month_start, s.coverage_start) AS period_start,
           LEAST(month_start + INTERVAL '1 month', NOW()) AS period_end
) b
WHERE v.annual_insurance_cost > 0
  AND b.period_end > b.period_start
  AND NOT EXISTS (SELECT 1 FROM maintenance_expenses me WHERE me.vehicle_id = v.id AND me.category = 'INSURANCE')

UNION ALL
-- Vehicle purchase, net of incentives
SELECT v.id,
       v.purchase_date::timestamptz,
       'ACQUISITION',
       'vehicles',
       v.id,
       v.purchase_price - COALESCE(v.purchase_incentives, 0)
FROM vehicles v
WHERE v.acquisition_type = 'PURCHASE' AND v.purchase_price > 0 AND v.purchase_date IS NOT NULL;
