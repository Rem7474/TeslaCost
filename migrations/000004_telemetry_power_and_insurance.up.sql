-- ============================================================================
-- TeslaCost Telemetry Power & Real Insurance Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- 1. Add TeslaMate power telemetry (acceleration peaks and regenerative braking) to drives
ALTER TABLE drives ADD COLUMN IF NOT EXISTS power_max INT;
ALTER TABLE drives ADD COLUMN IF NOT EXISTS power_min INT;
ALTER TABLE drives ADD COLUMN IF NOT EXISTS speed_max INT;

-- 2. Add vehicle-level real insurance configuration
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS annual_insurance_cost NUMERIC(10, 2);
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS annual_expected_mileage NUMERIC(10, 2) DEFAULT 15000;
