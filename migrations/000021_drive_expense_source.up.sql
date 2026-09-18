-- ============================================================================
-- TeslaCost Drive Expense Source Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

ALTER TABLE drive_expenses
    ADD COLUMN IF NOT EXISTS source VARCHAR(20) NOT NULL DEFAULT 'MANUAL'
    CHECK (source IN ('MANUAL', 'AUTO_TOLL'));
