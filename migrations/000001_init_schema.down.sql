-- ============================================================================
-- TeslaCost Initial Database Schema Migration (Down)
-- ============================================================================

DROP TABLE IF EXISTS charge_logs CASCADE;
DROP TABLE IF EXISTS maintenance_expenses CASCADE;
DROP TABLE IF EXISTS tire_rotations CASCADE;
DROP TABLE IF EXISTS tire_logs CASCADE;
DROP TABLE IF EXISTS tires CASCADE;
DROP TABLE IF EXISTS drive_expenses CASCADE;
DROP TABLE IF EXISTS trip_group_drives CASCADE;
DROP TABLE IF EXISTS trip_groups CASCADE;
DROP TABLE IF EXISTS drives CASCADE;
DROP TABLE IF EXISTS vehicles CASCADE;
DROP TABLE IF EXISTS users CASCADE;

DROP TYPE IF EXISTS auth_mode CASCADE;
DROP TYPE IF EXISTS tire_season CASCADE;
DROP TYPE IF EXISTS tire_position CASCADE;
