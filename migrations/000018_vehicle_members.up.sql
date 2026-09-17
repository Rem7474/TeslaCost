-- ============================================================================
-- TeslaCost Shared Vehicle Access (vehicle_members) Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

CREATE TABLE IF NOT EXISTS vehicle_members (
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(20) NOT NULL CHECK (role IN ('OWNER', 'EDITOR', 'VIEWER')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (vehicle_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_vehicle_members_user ON vehicle_members(user_id);
CREATE INDEX IF NOT EXISTS idx_vehicle_members_vehicle ON vehicle_members(vehicle_id);

-- Backfill: Every existing vehicle creator becomes an OWNER in vehicle_members
INSERT INTO vehicle_members (vehicle_id, user_id, role)
SELECT id, user_id, 'OWNER'
FROM vehicles
ON CONFLICT (vehicle_id, user_id) DO NOTHING;
