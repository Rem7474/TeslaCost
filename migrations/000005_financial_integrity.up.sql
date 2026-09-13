-- ============================================================================
-- TeslaCost Financial Integrity Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- 1. Synchronization state (full history import completion per resource)
CREATE TABLE IF NOT EXISTS sync_state (
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    resource VARCHAR(20) NOT NULL, -- 'drives' | 'charges'
    full_import_completed_at TIMESTAMPTZ,
    last_success_at TIMESTAMPTZ,
    PRIMARY KEY (vehicle_id, resource)
);

-- 2. Charges: unknown cost is NULL (never 0), cost origin is tracked
ALTER TABLE charge_logs ALTER COLUMN cost DROP NOT NULL;
ALTER TABLE charge_logs ALTER COLUMN cost DROP DEFAULT;
ALTER TABLE charge_logs ADD COLUMN IF NOT EXISTS cost_source VARCHAR(20) NOT NULL DEFAULT 'TESLAMATE'; -- TESLAMATE | MANUAL
UPDATE charge_logs SET cost_source = 'MANUAL' WHERE is_manual = TRUE;
ALTER TABLE charge_logs ADD COLUMN IF NOT EXISTS notes TEXT;

-- 3. Foreign currencies: conversion rate to EUR captured at input time
ALTER TABLE drive_expenses ADD COLUMN IF NOT EXISTS fx_rate NUMERIC(12, 6);
ALTER TABLE maintenance_expenses ADD COLUMN IF NOT EXISTS fx_rate NUMERIC(12, 6);
ALTER TABLE charge_logs ADD COLUMN IF NOT EXISTS fx_rate NUMERIC(12, 6);

-- 4. Recurring expenses: optional end of recurrence (contract termination)
ALTER TABLE maintenance_expenses ADD COLUMN IF NOT EXISTS recurrence_end_date TIMESTAMPTZ;

-- 5. Tires: distance already driven before entering TeslaCost (used tires)
ALTER TABLE tires ADD COLUMN IF NOT EXISTS initial_distance_km NUMERIC(10, 2) NOT NULL DEFAULT 0;
UPDATE tires t
SET initial_distance_km = GREATEST(
    t.accumulated_distance_km - COALESCE((
        SELECT SUM(s.distance_km)
        FROM tire_mount_sessions s
        WHERE s.tire_id = t.id AND s.dismounted_date IS NOT NULL
    ), 0),
    0
);

-- 6. Drive expenses: a single link (drive OR trip group)
UPDATE drive_expenses SET drive_id = NULL WHERE drive_id IS NOT NULL AND trip_group_id IS NOT NULL;
ALTER TABLE drive_expenses DROP CONSTRAINT IF EXISTS chk_drive_expense_single_link;
ALTER TABLE drive_expenses ADD CONSTRAINT chk_drive_expense_single_link
    CHECK (drive_id IS NULL OR trip_group_id IS NULL);

-- 7. Drives: explicit "no toll" qualification
ALTER TABLE drives ADD COLUMN IF NOT EXISTS toll_reviewed_at TIMESTAMPTZ;

-- 8. Indexes for link lookups and ON DELETE SET NULL cascades
CREATE INDEX IF NOT EXISTS idx_drive_expenses_drive ON drive_expenses(drive_id) WHERE drive_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_drive_expenses_trip_group ON drive_expenses(trip_group_id) WHERE trip_group_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_carpool_trips_drive ON carpool_trips(drive_id) WHERE drive_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_carpool_trips_trip_group ON carpool_trips(trip_group_id) WHERE trip_group_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_charges_missing_cost ON charge_logs(vehicle_id) WHERE cost IS NULL;
