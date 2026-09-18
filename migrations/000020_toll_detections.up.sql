-- ============================================================================
-- TeslaCost Toll Detections Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

CREATE TABLE IF NOT EXISTS toll_detections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    drive_id UUID NOT NULL UNIQUE REFERENCES drives(id) ON DELETE CASCADE,
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    segments JSONB NOT NULL,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_toll_detections_vehicle ON toll_detections(vehicle_id);
