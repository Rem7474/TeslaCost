-- ============================================================================
-- TeslaCost Upstream Deletions Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- Records deleted in TeslaMate are kept (tags, expense links) but excluded from every calculation.
ALTER TABLE drives ADD COLUMN IF NOT EXISTS deleted_upstream_at TIMESTAMPTZ;
ALTER TABLE charge_logs ADD COLUMN IF NOT EXISTS deleted_upstream_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_drives_active_vehicle_start ON drives(vehicle_id, start_time DESC) WHERE deleted_upstream_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_charges_active_vehicle_date ON charge_logs(vehicle_id, date DESC) WHERE deleted_upstream_at IS NULL;
