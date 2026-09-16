CREATE TABLE IF NOT EXISTS maintenance_reminders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    title VARCHAR(100) NOT NULL,
    category VARCHAR(50) NOT NULL DEFAULT 'MAINTENANCE',
    interval_km INT,
    interval_months INT,
    last_service_odometer NUMERIC(10, 2),
    last_service_date DATE,
    lead_km INT NOT NULL DEFAULT 1000,
    lead_days INT NOT NULL DEFAULT 30,
    webhook_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_notified_at TIMESTAMPTZ,
    last_notified_odometer NUMERIC(10, 2),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_maintenance_reminders_vehicle_id ON maintenance_reminders(vehicle_id);

CREATE TABLE IF NOT EXISTS vehicle_webhooks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE UNIQUE,
    url TEXT NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'GENERIC',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_vehicle_webhooks_vehicle_id ON vehicle_webhooks(vehicle_id);
