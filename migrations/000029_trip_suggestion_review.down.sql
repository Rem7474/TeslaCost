-- ============================================================================
-- TeslaCost Trip Suggestion Review Migration (Down)
-- ============================================================================

ALTER TABLE drives DROP COLUMN IF EXISTS trip_reviewed_at;
