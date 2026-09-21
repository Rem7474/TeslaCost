-- ============================================================================
-- TeslaCost Reset Toll Detections Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- The detection radius around a toll gate went from 150 m to 50 m. The stored results are a cache of what the
-- detection found with the old radius (and feed the "to qualify" queue): drop them, a new detection recomputes them.
-- Tolls already applied to drives are expenses and are not touched.
DELETE FROM toll_detections WHERE detected_at < NOW();
