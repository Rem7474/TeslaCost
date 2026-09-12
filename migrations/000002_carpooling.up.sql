-- ============================================================================
-- TeslaCost Carpooling / BlaBlaCar Module (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- 1. Carpool Trips table
CREATE TABLE IF NOT EXISTS carpool_trips (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    drive_id UUID REFERENCES drives(id) ON DELETE SET NULL,
    trip_group_id UUID REFERENCES trip_groups(id) ON DELETE SET NULL,
    title VARCHAR(150) NOT NULL,
    date TIMESTAMPTZ NOT NULL,
    distance_km NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    
    -- Real Cost Breakdown Components
    electricity_cost NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    tolls_cost NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    tires_cost NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    maintenance_cost NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    insurance_cost NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    other_cost NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    total_cost NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    
    -- Passenger Revenues & Financial Balance
    total_revenue NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    net_cost NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Carpool Passengers / Legs ("Bouts de trajet")
CREATE TABLE IF NOT EXISTS carpool_passengers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    carpool_trip_id UUID NOT NULL REFERENCES carpool_trips(id) ON DELETE CASCADE,
    passenger_name VARCHAR(100) NOT NULL,
    origin VARCHAR(150),
    destination VARCHAR(150),
    seats INT NOT NULL DEFAULT 1,
    amount_paid NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Optimized Indexes
CREATE INDEX IF NOT EXISTS idx_carpool_trips_vehicle_date ON carpool_trips(vehicle_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_carpool_passengers_trip ON carpool_passengers(carpool_trip_id);
