-- ============================================================================
-- TeslaCost Estimated Energy Naming Migration (Down)
-- ============================================================================

ALTER TABLE vehicles RENAME COLUMN estimated_kwh_100km TO pre_teslamate_kwh_100km;
ALTER TABLE vehicles RENAME COLUMN estimated_price_per_kwh TO pre_teslamate_eur_per_kwh;
