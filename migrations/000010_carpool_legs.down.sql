-- ============================================================================
-- TeslaCost Carpool Legs Migration (Down)
-- ============================================================================

ALTER TABLE carpool_passengers DROP CONSTRAINT IF EXISTS chk_carpool_passenger_stops;
ALTER TABLE carpool_passengers DROP COLUMN IF EXISTS alight_stop_index;
ALTER TABLE carpool_passengers DROP COLUMN IF EXISTS board_stop_index;

DROP TABLE IF EXISTS carpool_legs;
