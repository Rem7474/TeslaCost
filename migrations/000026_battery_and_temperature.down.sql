-- ============================================================================
-- TeslaCost Battery Level, Temperature and Battery Health Migration (Down)
-- Database: PostgreSQL 14+
-- ============================================================================

DROP TABLE IF EXISTS battery_health_snapshots;

ALTER TABLE drives DROP COLUMN IF EXISTS outside_temp_c;
ALTER TABLE drives DROP COLUMN IF EXISTS end_battery_level;
ALTER TABLE drives DROP COLUMN IF EXISTS start_battery_level;

ALTER TABLE charge_logs DROP COLUMN IF EXISTS outside_temp_c;
ALTER TABLE charge_logs DROP COLUMN IF EXISTS end_battery_level;
ALTER TABLE charge_logs DROP COLUMN IF EXISTS start_battery_level;
