-- ============================================================================
-- TeslaCost Expense Documents & Invoices (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

CREATE TABLE IF NOT EXISTS expense_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    file_size BIGINT NOT NULL,
    data BYTEA NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_expense_documents_vehicle ON expense_documents(vehicle_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_expense_documents_user ON expense_documents(user_id);

ALTER TABLE drive_expenses
    ADD COLUMN IF NOT EXISTS document_id UUID REFERENCES expense_documents(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_drive_expenses_document ON drive_expenses(document_id);

ALTER TABLE maintenance_expenses
    ADD COLUMN IF NOT EXISTS document_id UUID REFERENCES expense_documents(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_maintenance_expenses_document ON maintenance_expenses(document_id);

ALTER TABLE charge_logs
    ADD COLUMN IF NOT EXISTS document_id UUID REFERENCES expense_documents(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_charge_logs_document ON charge_logs(document_id);
