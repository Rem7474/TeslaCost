-- ============================================================================
-- TeslaCost Cost Ledger & Vehicle Acquisition Migration (Down)
-- ============================================================================

DROP VIEW IF EXISTS cost_ledger;

ALTER TABLE vehicles DROP CONSTRAINT IF EXISTS chk_vehicle_acquisition_type;
ALTER TABLE vehicles DROP COLUMN IF EXISTS expected_holding_months;
ALTER TABLE vehicles DROP COLUMN IF EXISTS expected_resale_value;
ALTER TABLE vehicles DROP COLUMN IF EXISTS purchase_incentives;
ALTER TABLE vehicles DROP COLUMN IF EXISTS purchase_odometer;
ALTER TABLE vehicles DROP COLUMN IF EXISTS purchase_date;
ALTER TABLE vehicles DROP COLUMN IF EXISTS purchase_price;
ALTER TABLE vehicles DROP COLUMN IF EXISTS acquisition_type;
