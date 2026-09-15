-- ============================================================================
-- TeslaCost Expense Documents & Invoices (Down)
-- Database: PostgreSQL 14+
-- ============================================================================

ALTER TABLE charge_logs DROP COLUMN IF EXISTS document_id;
ALTER TABLE maintenance_expenses DROP COLUMN IF EXISTS document_id;
ALTER TABLE drive_expenses DROP COLUMN IF EXISTS document_id;
DROP TABLE IF EXISTS expense_documents;
