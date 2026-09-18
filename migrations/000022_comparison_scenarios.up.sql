-- ============================================================================
-- TeslaCost EV vs ICE Cost Comparison Scenarios Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

CREATE TABLE IF NOT EXISTS comparison_scenarios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    vehicle_id UUID REFERENCES vehicles(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    mode VARCHAR(20) NOT NULL CHECK (mode IN ('RETROSPECTIVE', 'PROJECTION')),
    annual_km NUMERIC(10, 0) NOT NULL CHECK (annual_km > 0),
    years INT NOT NULL CHECK (years BETWEEN 1 AND 15),
    ice_fuel_type VARCHAR(20) NOT NULL CHECK (ice_fuel_type IN ('SP95_E10', 'SP98', 'DIESEL', 'E85', 'GPL')),
    ice_l_100km NUMERIC(5, 2) NOT NULL CHECK (ice_l_100km > 0),
    ice_fuel_price NUMERIC(6, 3) NOT NULL CHECK (ice_fuel_price >= 0),
    ice_purchase_price NUMERIC(10, 2) NOT NULL CHECK (ice_purchase_price >= 0),
    ice_resale_value NUMERIC(10, 2) NOT NULL DEFAULT 0 CHECK (ice_resale_value >= 0),
    ice_maintenance_yearly NUMERIC(10, 2) NOT NULL CHECK (ice_maintenance_yearly >= 0),
    ice_insurance_yearly NUMERIC(10, 2) NOT NULL CHECK (ice_insurance_yearly >= 0),
    ice_tax_yearly NUMERIC(10, 2) NOT NULL DEFAULT 0 CHECK (ice_tax_yearly >= 0),
    ev_inputs JSONB,
    options JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_comparison_mode_inputs CHECK (
        (mode = 'RETROSPECTIVE' AND vehicle_id IS NOT NULL)
        OR (mode = 'PROJECTION' AND ev_inputs IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_comparison_scenarios_user_id ON comparison_scenarios(user_id);
CREATE INDEX IF NOT EXISTS idx_comparison_scenarios_vehicle_id ON comparison_scenarios(vehicle_id);
