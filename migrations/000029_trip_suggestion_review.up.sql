-- ============================================================================
-- TeslaCost Trip Suggestion Review Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- Set when the user rules that a drive is not part of a trip: the automatic trip detection stops suggesting it.
ALTER TABLE drives ADD COLUMN IF NOT EXISTS trip_reviewed_at TIMESTAMPTZ;
