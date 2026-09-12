-- ============================================================================
-- TeslaCost Tire Enhancements & Lifecycle Sessions Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- 1. Add odometer tracking and estimated lifespan to tires
ALTER TABLE tires ADD COLUMN IF NOT EXISTS mounted_odometer NUMERIC(10, 2);
ALTER TABLE tires ADD COLUMN IF NOT EXISTS accumulated_distance_km NUMERIC(10, 2) NOT NULL DEFAULT 0.0;
ALTER TABLE tires ADD COLUMN IF NOT EXISTS estimated_lifespan_km INT NOT NULL DEFAULT 40000;

-- 2. Tire Mount/Dismount Sessions (Full Lifecycle History)
CREATE TABLE IF NOT EXISTS tire_mount_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tire_id UUID NOT NULL REFERENCES tires(id) ON DELETE CASCADE,
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    position tire_position NOT NULL,
    mounted_date TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    mounted_odometer NUMERIC(10, 2) NOT NULL,
    dismounted_date TIMESTAMPTZ,
    dismounted_odometer NUMERIC(10, 2),
    distance_km NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Optimized Indexes
CREATE INDEX IF NOT EXISTS idx_tire_mount_sessions_tire ON tire_mount_sessions(tire_id, mounted_date DESC);
CREATE INDEX IF NOT EXISTS idx_tire_mount_sessions_vehicle ON tire_mount_sessions(vehicle_id);
