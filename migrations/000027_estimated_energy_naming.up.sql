-- ============================================================================
-- TeslaCost Estimated Energy Naming Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- The energy estimate for kilometres without recorded charging is not tied to TeslaMate, and its price is
-- expressed in the reference currency rather than in euros.
ALTER TABLE vehicles RENAME COLUMN pre_teslamate_kwh_100km TO estimated_kwh_100km;
ALTER TABLE vehicles RENAME COLUMN pre_teslamate_eur_per_kwh TO estimated_price_per_kwh;
