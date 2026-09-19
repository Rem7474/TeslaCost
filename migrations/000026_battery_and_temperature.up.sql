-- ============================================================================
-- TeslaCost Battery Level, Temperature and Battery Health Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- State of charge (%) at the start and end of each session, and the average outside temperature (Celsius).
-- NULL for manual entries and for records imported before this migration.
ALTER TABLE charge_logs ADD COLUMN IF NOT EXISTS start_battery_level SMALLINT;
ALTER TABLE charge_logs ADD COLUMN IF NOT EXISTS end_battery_level SMALLINT;
ALTER TABLE charge_logs ADD COLUMN IF NOT EXISTS outside_temp_c NUMERIC(4, 1);

ALTER TABLE drives ADD COLUMN IF NOT EXISTS start_battery_level SMALLINT;
ALTER TABLE drives ADD COLUMN IF NOT EXISTS end_battery_level SMALLINT;
ALTER TABLE drives ADD COLUMN IF NOT EXISTS outside_temp_c NUMERIC(4, 1);

-- Battery health as computed by TeslaMate, kept once a day to draw its evolution.
CREATE TABLE IF NOT EXISTS battery_health_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    captured_on DATE NOT NULL,
    max_capacity_kwh NUMERIC(6, 2),
    current_capacity_kwh NUMERIC(6, 2),
    health_percent NUMERIC(5, 2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_battery_health_vehicle_day UNIQUE (vehicle_id, captured_on)
);

-- Records imported so far lack the new fields: the next synchronization re-reads the whole TeslaMate history
-- (an interrupted import resumes on the following one). Existing costs entered by hand are preserved.
UPDATE sync_state SET full_import_completed_at = NULL WHERE resource IN ('drives', 'charges');
