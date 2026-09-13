-- ============================================================================
-- TeslaCost Vehicle Ownership, Financing & Insurance Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- 1. Ownership contract of a vehicle: cash purchase, loan, LOA (lease with purchase option) or LLD (long-term rental)
CREATE TABLE IF NOT EXISTS vehicle_ownership (
    vehicle_id UUID PRIMARY KEY REFERENCES vehicles(id) ON DELETE CASCADE,
    acquisition_type VARCHAR(10) NOT NULL CHECK (acquisition_type IN ('CASH', 'LOAN', 'LOA', 'LLD')),
    start_date DATE NOT NULL,
    start_odometer NUMERIC(10, 2),

    -- Purchase (CASH, LOAN)
    purchase_price NUMERIC(10, 2),
    purchase_fees NUMERIC(10, 2),          -- Registration, malus, delivery / preparation fees
    incentives NUMERIC(10, 2),             -- Ecological bonus, conversion premium, discounts
    expected_resale_value NUMERIC(10, 2),  -- Also used after a LOA purchase option is exercised
    expected_holding_months INT,

    -- Loan (LOAN)
    loan_amount NUMERIC(10, 2),
    loan_rate_pct NUMERIC(6, 3),           -- Annual nominal rate
    loan_duration_months INT,
    loan_fees NUMERIC(10, 2),
    loan_insurance_monthly NUMERIC(10, 2), -- Borrower insurance

    -- Lease (LOA, LLD)
    lease_down_payment NUMERIC(10, 2),     -- "Apport" / first increased rent
    lease_monthly_rent NUMERIC(10, 2),
    lease_duration_months INT,
    lease_fees NUMERIC(10, 2),             -- Application fees
    lease_deposit NUMERIC(10, 2),          -- Refundable security deposit (not a cost)
    lease_km_allowance_per_year NUMERIC(10, 2),
    lease_excess_km_price NUMERIC(6, 3),   -- EUR per kilometer above the allowance
    lease_end_fees_estimate NUMERIC(10, 2),-- Return / refurbishment fees
    lease_purchase_option_price NUMERIC(10, 2),
    lease_includes_maintenance BOOLEAN NOT NULL DEFAULT FALSE,
    lease_includes_insurance BOOLEAN NOT NULL DEFAULT FALSE,
    lease_includes_tires BOOLEAN NOT NULL DEFAULT FALSE,
    option_exercised_date DATE,

    -- End of ownership (sale, return)
    end_date DATE,
    sale_price NUMERIC(10, 2),

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO vehicle_ownership (
    vehicle_id, acquisition_type, start_date, start_odometer,
    purchase_price, incentives, expected_resale_value, expected_holding_months
)
SELECT v.id,
       CASE v.acquisition_type WHEN 'PURCHASE' THEN 'CASH' ELSE 'LOA' END,
       COALESCE(v.purchase_date, v.created_at::date),
       v.purchase_odometer,
       v.purchase_price, v.purchase_incentives, v.expected_resale_value, v.expected_holding_months
FROM vehicles v
WHERE v.acquisition_type IS NOT NULL
ON CONFLICT (vehicle_id) DO NOTHING;

-- 2. Insurance is a recorded cost, not a vehicle setting: the annual premium becomes a monthly recurring expense
--    starting when the vehicle coverage started.
INSERT INTO maintenance_expenses (vehicle_id, category, amount, currency, date, is_recurring, recurrence_interval_months, description)
SELECT v.id,
       'INSURANCE',
       ROUND(v.annual_insurance_cost / 12, 2),
       'EUR',
       LEAST(
           v.created_at,
           COALESCE(v.purchase_date::timestamptz, v.created_at),
           COALESCE((SELECT MIN(d.start_time) FROM drives d WHERE d.vehicle_id = v.id AND d.deleted_upstream_at IS NULL), v.created_at)
       ),
       TRUE,
       1,
       'Prime d''assurance mensualisée (reprise de la fiche véhicule)'
FROM vehicles v
WHERE v.annual_insurance_cost > 0
  AND NOT EXISTS (SELECT 1 FROM maintenance_expenses me WHERE me.vehicle_id = v.id AND me.category = 'INSURANCE');

-- 3. Former vehicle columns
DROP VIEW IF EXISTS cost_ledger;
ALTER TABLE vehicles DROP CONSTRAINT IF EXISTS chk_vehicle_acquisition_type;
ALTER TABLE vehicles DROP COLUMN IF EXISTS acquisition_type;
ALTER TABLE vehicles DROP COLUMN IF EXISTS purchase_price;
ALTER TABLE vehicles DROP COLUMN IF EXISTS purchase_date;
ALTER TABLE vehicles DROP COLUMN IF EXISTS purchase_odometer;
ALTER TABLE vehicles DROP COLUMN IF EXISTS purchase_incentives;
ALTER TABLE vehicles DROP COLUMN IF EXISTS expected_resale_value;
ALTER TABLE vehicles DROP COLUMN IF EXISTS expected_holding_months;
ALTER TABLE vehicles DROP COLUMN IF EXISTS annual_insurance_cost;
ALTER TABLE vehicles DROP COLUMN IF EXISTS annual_expected_mileage;

-- 4. Cost ledger: every cash cost of a vehicle, in EUR, as dated entries.
--    Recurring expenses stop at their end date or at the end of ownership (end date included).
CREATE VIEW cost_ledger AS
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
-- Maintenance, insurance and fixed expenses, recurring ones expanded
SELECT m.vehicle_id,
       occ,
       m.category,
       'maintenance_expenses',
       m.id,
       ROUND(CASE WHEN m.currency = 'EUR' THEN m.amount ELSE m.amount * m.fx_rate END, 2)
FROM maintenance_expenses m
LEFT JOIN vehicle_ownership o ON o.vehicle_id = m.vehicle_id
CROSS JOIN LATERAL generate_series(
    m.date,
    CASE
        WHEN m.is_recurring AND COALESCE(m.recurrence_interval_months, 0) > 0
        THEN GREATEST(m.date, LEAST(NOW(), COALESCE(m.recurrence_end_date, 'infinity'::timestamptz),
                                    COALESCE((o.end_date + 1)::timestamptz - INTERVAL '1 microsecond', 'infinity'::timestamptz)))
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
-- Purchase (cash or loan): price and acquisition fees, net of incentives
SELECT o.vehicle_id,
       o.start_date::timestamptz,
       'ACQUISITION',
       'vehicle_ownership',
       o.vehicle_id,
       o.purchase_price + COALESCE(o.purchase_fees, 0) - COALESCE(o.incentives, 0)
FROM vehicle_ownership o
WHERE o.acquisition_type IN ('CASH', 'LOAN') AND o.purchase_price > 0

UNION ALL
-- LOA purchase option exercised
SELECT o.vehicle_id,
       o.option_exercised_date::timestamptz,
       'ACQUISITION',
       'vehicle_ownership',
       o.vehicle_id,
       o.lease_purchase_option_price
FROM vehicle_ownership o
WHERE o.acquisition_type = 'LOA' AND o.option_exercised_date IS NOT NULL
  AND o.lease_purchase_option_price > 0 AND o.option_exercised_date <= CURRENT_DATE

UNION ALL
-- One-off financing fees: loan fees, lease down payment and application fees at contract start
SELECT o.vehicle_id,
       o.start_date::timestamptz,
       'FINANCING',
       'vehicle_ownership',
       o.vehicle_id,
       CASE WHEN o.acquisition_type = 'LOAN' THEN COALESCE(o.loan_fees, 0)
            ELSE COALESCE(o.lease_down_payment, 0) + COALESCE(o.lease_fees, 0) END
FROM vehicle_ownership o
WHERE o.start_date <= CURRENT_DATE
  AND ((o.acquisition_type = 'LOAN' AND o.loan_fees > 0)
    OR (o.acquisition_type IN ('LOA', 'LLD') AND COALESCE(o.lease_down_payment, 0) + COALESCE(o.lease_fees, 0) > 0))

UNION ALL
-- Loan monthly interest (annuity schedule, first payment one month after start) and borrower insurance
SELECT o.vehicle_id,
       (o.start_date + make_interval(months => k))::timestamptz,
       'FINANCING',
       'vehicle_ownership',
       o.vehicle_id,
       ROUND(
           CASE WHEN COALESCE(o.loan_rate_pct, 0) = 0 THEN 0
                ELSE (o.loan_amount * power(1 + o.loan_rate_pct / 1200, k - 1)
                      - (o.loan_amount * (o.loan_rate_pct / 1200) / (1 - power(1 + o.loan_rate_pct / 1200, -o.loan_duration_months)))
                        * (power(1 + o.loan_rate_pct / 1200, k - 1) - 1) / (o.loan_rate_pct / 1200))
                     * (o.loan_rate_pct / 1200)
           END + COALESCE(o.loan_insurance_monthly, 0),
       2)
FROM vehicle_ownership o
CROSS JOIN LATERAL generate_series(1, o.loan_duration_months) AS k
WHERE o.acquisition_type = 'LOAN'
  AND o.loan_amount > 0 AND o.loan_duration_months > 0
  AND o.start_date + make_interval(months => k) <= LEAST(CURRENT_DATE, COALESCE(o.end_date, CURRENT_DATE))

UNION ALL
-- Lease monthly rents (first rent at contract start), until the contract ends, the vehicle is returned or the option is exercised
SELECT o.vehicle_id,
       (o.start_date + make_interval(months => k))::timestamptz,
       'FINANCING',
       'vehicle_ownership',
       o.vehicle_id,
       o.lease_monthly_rent
FROM vehicle_ownership o
CROSS JOIN LATERAL generate_series(0, o.lease_duration_months - 1) AS k
WHERE o.acquisition_type IN ('LOA', 'LLD')
  AND o.lease_monthly_rent > 0 AND o.lease_duration_months > 0
  AND o.start_date + make_interval(months => k)
      < LEAST(CURRENT_DATE + 1, COALESCE(o.end_date, 'infinity'::date), COALESCE(o.option_exercised_date, 'infinity'::date))

UNION ALL
-- Lease return fees, once the contract is over without exercising the option
SELECT o.vehicle_id,
       LEAST(COALESCE(o.end_date, 'infinity'::date), (o.start_date + make_interval(months => o.lease_duration_months))::date)::timestamptz,
       'FINANCING',
       'vehicle_ownership',
       o.vehicle_id,
       o.lease_end_fees_estimate
FROM vehicle_ownership o
WHERE o.acquisition_type IN ('LOA', 'LLD')
  AND o.lease_end_fees_estimate > 0
  AND o.option_exercised_date IS NULL
  AND LEAST(COALESCE(o.end_date, 'infinity'::date), (o.start_date + make_interval(months => o.lease_duration_months))::date) <= CURRENT_DATE;
