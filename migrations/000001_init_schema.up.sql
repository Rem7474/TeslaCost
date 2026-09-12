-- ============================================================================
-- TeslaCost Initial Database Schema Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- Extensions for UUID support
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Custom Enumerations
CREATE TYPE tire_position AS ENUM ('FL', 'FR', 'RL', 'RR', 'STORAGE', 'DISPOSED');
CREATE TYPE tire_season AS ENUM ('SUMMER', 'WINTER', 'ALL_SEASON');
CREATE TYPE auth_mode AS ENUM ('BEARER', 'BASIC', 'NONE');

-- 1. Users table (Local JWT authentication)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 2. Vehicles table (Multi-vehicle management & optional TeslaMate API link)
CREATE TABLE IF NOT EXISTS vehicles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    vin VARCHAR(30),
    teslamate_car_id INT,
    current_odometer NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    teslamate_api_url VARCHAR(255),
    teslamate_auth_type auth_mode NOT NULL DEFAULT 'NONE',
    teslamate_api_key_encrypted TEXT,
    teslamate_basic_user VARCHAR(100),
    teslamate_basic_pass_encrypted TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_user_teslamate_car UNIQUE (user_id, teslamate_car_id)
);

-- 3. Drives table (Synchronized from TeslaMate or created manually)
CREATE TABLE IF NOT EXISTS drives (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    teslamate_drive_id INT,
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL,
    start_odometer NUMERIC(10, 2),
    end_odometer NUMERIC(10, 2),
    distance_km NUMERIC(8, 2) NOT NULL,
    duration_min INT NOT NULL DEFAULT 0,
    speed_avg NUMERIC(5, 2),
    start_address TEXT,
    end_address TEXT,
    energy_consumed_kwh NUMERIC(8, 2),
    consumption_kwh_100km NUMERIC(6, 2),
    tags TEXT[] NOT NULL DEFAULT '{}',
    is_manual BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_vehicle_teslamate_drive UNIQUE (vehicle_id, teslamate_drive_id)
);

-- 4. Trip Groups table (Grouping consecutive drives / long road trips)
CREATE TABLE IF NOT EXISTS trip_groups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    name VARCHAR(150) NOT NULL,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Join table for trip groups and drives
CREATE TABLE IF NOT EXISTS trip_group_drives (
    trip_group_id UUID NOT NULL REFERENCES trip_groups(id) ON DELETE CASCADE,
    drive_id UUID NOT NULL REFERENCES drives(id) ON DELETE CASCADE,
    order_index INT NOT NULL DEFAULT 0,
    PRIMARY KEY (trip_group_id, drive_id)
);

-- 5. Drive Expenses table (Tolls, parkings, travel costs associated to drive or trip group)
CREATE TABLE IF NOT EXISTS drive_expenses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    trip_group_id UUID REFERENCES trip_groups(id) ON DELETE SET NULL,
    drive_id UUID REFERENCES drives(id) ON DELETE SET NULL,
    type VARCHAR(50) NOT NULL, -- 'TOLL', 'PARKING', 'FERRY', 'OTHER'
    amount NUMERIC(10, 2) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'EUR',
    date TIMESTAMPTZ NOT NULL,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 6. Tires table (Tire lifecycle management)
CREATE TABLE IF NOT EXISTS tires (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID REFERENCES vehicles(id) ON DELETE SET NULL,
    brand VARCHAR(100) NOT NULL,
    model VARCHAR(100) NOT NULL,
    dimension VARCHAR(50) NOT NULL, -- e.g. '235/40 R19 96W'
    season tire_season NOT NULL DEFAULT 'SUMMER',
    purchase_date DATE NOT NULL,
    purchase_price NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    current_position tire_position NOT NULL DEFAULT 'STORAGE',
    initial_depth_mm NUMERIC(4, 2) NOT NULL DEFAULT 8.0,
    min_legal_depth_mm NUMERIC(4, 2) NOT NULL DEFAULT 1.6,
    dot_code VARCHAR(10),
    is_archived BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tire Wear Measurements (Depth logs)
CREATE TABLE IF NOT EXISTS tire_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tire_id UUID NOT NULL REFERENCES tires(id) ON DELETE CASCADE,
    date TIMESTAMPTZ NOT NULL,
    odometer NUMERIC(10, 2) NOT NULL,
    depth_mm NUMERIC(4, 2) NOT NULL,
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Tire Rotations History
CREATE TABLE IF NOT EXISTS tire_rotations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    date TIMESTAMPTZ NOT NULL,
    odometer NUMERIC(10, 2) NOT NULL,
    mapping_json JSONB NOT NULL, -- e.g. {"FL": "tire-uuid-1", "FR": "tire-uuid-2", ...}
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 7. Maintenance & Fixed Expenses
CREATE TABLE IF NOT EXISTS maintenance_expenses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    category VARCHAR(50) NOT NULL, -- 'MAINTENANCE', 'INSURANCE', 'SUBSCRIPTION', 'TAX', 'ACCESSORY', 'OTHER'
    amount NUMERIC(10, 2) NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'EUR',
    date TIMESTAMPTZ NOT NULL,
    odometer NUMERIC(10, 2),
    is_recurring BOOLEAN NOT NULL DEFAULT FALSE,
    recurrence_interval_months INT,
    description TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 8. Charging Logs (Energy costs)
CREATE TABLE IF NOT EXISTS charge_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    vehicle_id UUID NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    teslamate_charge_id INT,
    date TIMESTAMPTZ NOT NULL,
    end_date TIMESTAMPTZ,
    address TEXT,
    kwh_added NUMERIC(8, 3) NOT NULL,
    kwh_used NUMERIC(8, 3),
    cost NUMERIC(10, 2) NOT NULL DEFAULT 0.0,
    currency VARCHAR(3) NOT NULL DEFAULT 'EUR',
    odometer NUMERIC(10, 2),
    is_manual BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_vehicle_teslamate_charge UNIQUE (vehicle_id, teslamate_charge_id)
);

-- Optimized Indexes
CREATE INDEX IF NOT EXISTS idx_vehicles_user_id ON vehicles(user_id);
CREATE INDEX IF NOT EXISTS idx_drives_vehicle_start ON drives(vehicle_id, start_time DESC);
CREATE INDEX IF NOT EXISTS idx_drives_tags ON drives USING GIN(tags);
CREATE INDEX IF NOT EXISTS idx_trip_group_drives_group ON trip_group_drives(trip_group_id);
CREATE INDEX IF NOT EXISTS idx_trip_group_drives_drive ON trip_group_drives(drive_id);
CREATE INDEX IF NOT EXISTS idx_drive_expenses_vehicle_date ON drive_expenses(vehicle_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_tires_vehicle ON tires(vehicle_id);
CREATE INDEX IF NOT EXISTS idx_tire_logs_tire_date ON tire_logs(tire_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_tire_rotations_vehicle_date ON tire_rotations(vehicle_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_maintenance_vehicle_date ON maintenance_expenses(vehicle_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_charges_vehicle_date ON charge_logs(vehicle_id, date DESC);
