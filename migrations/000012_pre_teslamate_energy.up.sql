-- ============================================================================
-- TeslaCost Pre-TeslaMate Energy & Charging Estimation Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

ALTER TABLE vehicles
    ADD COLUMN IF NOT EXISTS pre_teslamate_kwh_100km NUMERIC(5, 2) DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS pre_teslamate_eur_per_kwh NUMERIC(6, 4) DEFAULT NULL;
