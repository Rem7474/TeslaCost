-- ============================================================================
-- TeslaCost Upstream Deletions Migration (Down)
-- ============================================================================

DROP INDEX IF EXISTS idx_charges_active_vehicle_date;
DROP INDEX IF EXISTS idx_drives_active_vehicle_start;

ALTER TABLE charge_logs DROP COLUMN IF EXISTS deleted_upstream_at;
ALTER TABLE drives DROP COLUMN IF EXISTS deleted_upstream_at;
