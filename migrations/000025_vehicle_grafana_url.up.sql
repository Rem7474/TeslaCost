-- ============================================================================
-- TeslaCost TeslaMate Grafana URL Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- Base URL of the Grafana instance that serves the TeslaMate dashboards; used to link a drive to its TeslaMate detail page.
ALTER TABLE vehicles ADD COLUMN IF NOT EXISTS teslamate_grafana_url TEXT;
