-- ============================================================================
-- TeslaCost TeslaMate Grafana URL Migration (Down)
-- Database: PostgreSQL 14+
-- ============================================================================

ALTER TABLE vehicles DROP COLUMN IF EXISTS teslamate_grafana_url;
