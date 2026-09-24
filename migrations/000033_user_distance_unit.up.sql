-- ============================================================================
-- TeslaCost User Distance Unit Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- Distances are always stored and computed in kilometers (TeslaMate, toll data and existing
-- history are all metric); this only controls what the frontend converts to for display and
-- form input. Per account, like language, since housemates sharing a vehicle may want different
-- units on the same data.
ALTER TABLE users ADD COLUMN distance_unit TEXT NOT NULL DEFAULT 'km' CHECK (distance_unit IN ('km', 'mi'));
