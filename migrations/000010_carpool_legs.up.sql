-- ============================================================================
-- TeslaCost Carpool Legs Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- 1. A carpool trip is an ordered list of legs (one per drive, or entered manually).
--    Stops are numbered 0..N: leg i goes from stop i to stop i + 1.
CREATE TABLE IF NOT EXISTS carpool_legs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    carpool_trip_id UUID NOT NULL REFERENCES carpool_trips(id) ON DELETE CASCADE,
    order_index INT NOT NULL,
    drive_id UUID REFERENCES drives(id) ON DELETE SET NULL,
    start_label VARCHAR(150),
    end_label VARCHAR(150),
    distance_km NUMERIC(10, 2) NOT NULL DEFAULT 0,
    electricity_cost NUMERIC(10, 2) NOT NULL DEFAULT 0,
    tolls_cost NUMERIC(10, 2) NOT NULL DEFAULT 0,
    tires_cost NUMERIC(10, 2) NOT NULL DEFAULT 0,
    maintenance_cost NUMERIC(10, 2) NOT NULL DEFAULT 0,
    insurance_cost NUMERIC(10, 2) NOT NULL DEFAULT 0,
    other_cost NUMERIC(10, 2) NOT NULL DEFAULT 0,
    CONSTRAINT uq_carpool_leg_order UNIQUE (carpool_trip_id, order_index)
);

CREATE INDEX IF NOT EXISTS idx_carpool_legs_drive ON carpool_legs(drive_id) WHERE drive_id IS NOT NULL;

-- Existing trips become a single leg carrying their cost breakdown
INSERT INTO carpool_legs (
    carpool_trip_id, order_index, drive_id, distance_km,
    electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost
)
SELECT t.id, 0, t.drive_id, t.distance_km,
       t.electricity_cost, t.tolls_cost, t.tires_cost, t.maintenance_cost, t.insurance_cost, t.other_cost
FROM carpool_trips t
WHERE NOT EXISTS (SELECT 1 FROM carpool_legs l WHERE l.carpool_trip_id = t.id);

-- 2. Passengers board and alight at stops; existing passengers ride the whole trip
ALTER TABLE carpool_passengers ADD COLUMN IF NOT EXISTS board_stop_index INT NOT NULL DEFAULT 0;
ALTER TABLE carpool_passengers ADD COLUMN IF NOT EXISTS alight_stop_index INT NOT NULL DEFAULT 1;
ALTER TABLE carpool_passengers DROP CONSTRAINT IF EXISTS chk_carpool_passenger_stops;
ALTER TABLE carpool_passengers ADD CONSTRAINT chk_carpool_passenger_stops
    CHECK (board_stop_index >= 0 AND alight_stop_index > board_stop_index);
