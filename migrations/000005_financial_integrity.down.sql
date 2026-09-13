-- ============================================================================
-- TeslaCost Financial Integrity Migration (Down)
-- ============================================================================

DROP INDEX IF EXISTS idx_charges_missing_cost;
DROP INDEX IF EXISTS idx_carpool_trips_trip_group;
DROP INDEX IF EXISTS idx_carpool_trips_drive;
DROP INDEX IF EXISTS idx_drive_expenses_trip_group;
DROP INDEX IF EXISTS idx_drive_expenses_drive;

ALTER TABLE drives DROP COLUMN IF EXISTS toll_reviewed_at;

ALTER TABLE drive_expenses DROP CONSTRAINT IF EXISTS chk_drive_expense_single_link;

ALTER TABLE tires DROP COLUMN IF EXISTS initial_distance_km;

ALTER TABLE maintenance_expenses DROP COLUMN IF EXISTS recurrence_end_date;

ALTER TABLE charge_logs DROP COLUMN IF EXISTS fx_rate;
ALTER TABLE maintenance_expenses DROP COLUMN IF EXISTS fx_rate;
ALTER TABLE drive_expenses DROP COLUMN IF EXISTS fx_rate;

ALTER TABLE charge_logs DROP COLUMN IF EXISTS notes;
ALTER TABLE charge_logs DROP COLUMN IF EXISTS cost_source;
UPDATE charge_logs SET cost = 0 WHERE cost IS NULL;
ALTER TABLE charge_logs ALTER COLUMN cost SET DEFAULT 0.0;
ALTER TABLE charge_logs ALTER COLUMN cost SET NOT NULL;

DROP TABLE IF EXISTS sync_state;
