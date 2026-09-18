-- ============================================================================
-- TeslaCost Optional Fill-up Odometer Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- A fill-up can be entered without a mileage; it is then estimated from the odometer readings.
ALTER TABLE fuel_logs ALTER COLUMN odometer DROP NOT NULL;
