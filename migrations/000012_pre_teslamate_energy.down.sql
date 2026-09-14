ALTER TABLE vehicles
    DROP COLUMN IF EXISTS pre_teslamate_kwh_100km,
    DROP COLUMN IF EXISTS pre_teslamate_eur_per_kwh;
