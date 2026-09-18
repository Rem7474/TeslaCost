-- ============================================================================
-- TeslaCost Optional Fill-up Odometer Migration (Down)
-- Database: PostgreSQL 14+
-- ============================================================================

-- Fill-ups without mileage take the last known mileage before them (0 when there is none).
UPDATE fuel_logs f
SET odometer = COALESCE(
    (SELECT p.odometer FROM fuel_logs p
      WHERE p.vehicle_id = f.vehicle_id AND p.odometer IS NOT NULL AND p.date <= f.date
      ORDER BY p.date DESC LIMIT 1),
    (SELECT c.odometer FROM odometer_checkpoints c
      WHERE c.vehicle_id = f.vehicle_id AND c.date <= f.date::date
      ORDER BY c.date DESC LIMIT 1),
    0)
WHERE f.odometer IS NULL;

ALTER TABLE fuel_logs ALTER COLUMN odometer SET NOT NULL;
