-- TeslaCost Tire Enhancements Migration (Down)
DROP TABLE IF EXISTS tire_mount_sessions;
ALTER TABLE tires DROP COLUMN IF EXISTS mounted_odometer;
ALTER TABLE tires DROP COLUMN IF EXISTS accumulated_distance_km;
ALTER TABLE tires DROP COLUMN IF EXISTS estimated_lifespan_km;
