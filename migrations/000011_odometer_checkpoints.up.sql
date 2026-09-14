-- ============================================================================
-- TeslaCost Odometer Checkpoints Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

CREATE TABLE IF NOT EXISTS odometer_checkpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    date DATE NOT NULL,
    odometer NUMERIC(10, 2) NOT NULL CHECK (odometer >= 0),
    notes VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_vehicle_checkpoint_date_odo UNIQUE (vehicle_id, date, odometer)
);

CREATE INDEX IF NOT EXISTS idx_odometer_checkpoints_vehicle_date ON odometer_checkpoints(vehicle_id, date);
