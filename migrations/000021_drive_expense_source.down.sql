-- ============================================================================
-- TeslaCost Drive Expense Source Migration (Down)
-- Database: PostgreSQL 14+
-- ============================================================================

ALTER TABLE drive_expenses DROP COLUMN IF EXISTS source;
