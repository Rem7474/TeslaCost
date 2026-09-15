-- ============================================================================
-- TeslaCost Maintenance Amortization & Closure (Down)
-- Database: PostgreSQL 14+
-- ============================================================================

ALTER TABLE maintenance_expenses
    DROP CONSTRAINT IF EXISTS chk_maintenance_amortization_mode,
    DROP COLUMN IF EXISTS amortization_mode,
    DROP COLUMN IF EXISTS coverage_km,
    DROP COLUMN IF EXISTS coverage_months,
    DROP COLUMN IF EXISTS closes_maintenance_id;
