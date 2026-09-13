-- ============================================================================
-- TeslaCost Telemetry Power & Real Insurance Migration (Down)
-- ============================================================================

ALTER TABLE drives DROP COLUMN IF EXISTS power_max;
ALTER TABLE drives DROP COLUMN IF EXISTS power_min;
ALTER TABLE drives DROP COLUMN IF EXISTS speed_max;

ALTER TABLE vehicles DROP COLUMN IF EXISTS annual_insurance_cost;
ALTER TABLE vehicles DROP COLUMN IF EXISTS annual_expected_mileage;
