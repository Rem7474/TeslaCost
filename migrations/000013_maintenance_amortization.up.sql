-- ============================================================================
-- TeslaCost Maintenance Amortization & Closure (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

ALTER TABLE maintenance_expenses
    ADD COLUMN IF NOT EXISTS amortization_mode VARCHAR(20) NOT NULL DEFAULT 'NONE',
    ADD COLUMN IF NOT EXISTS coverage_km NUMERIC(10, 2) DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS coverage_months INT DEFAULT NULL,
    ADD COLUMN IF NOT EXISTS closes_maintenance_id UUID REFERENCES maintenance_expenses(id) ON DELETE SET NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_maintenance_amortization_mode'
    ) THEN
        ALTER TABLE maintenance_expenses
            ADD CONSTRAINT chk_maintenance_amortization_mode
            CHECK (amortization_mode IN ('NONE', 'DISTANCE', 'DURATION', 'HYBRID'));
    END IF;
END $$;
