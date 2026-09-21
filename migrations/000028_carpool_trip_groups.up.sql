-- ============================================================================
-- TeslaCost Carpool Trip Groups Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- A carpool made of several drives is a trip: give the existing ones a trip group so they appear with the trips.
DO $$
DECLARE
    c RECORD;
    new_group UUID;
BEGIN
    FOR c IN
        SELECT t.id, t.vehicle_id, t.title
        FROM carpool_trips t
        WHERE t.trip_group_id IS NULL
          AND (SELECT COUNT(DISTINCT l.drive_id) FROM carpool_legs l WHERE l.carpool_trip_id = t.id AND l.drive_id IS NOT NULL) >= 2
    LOOP
        INSERT INTO trip_groups (vehicle_id, name) VALUES (c.vehicle_id, c.title) RETURNING id INTO new_group;

        INSERT INTO trip_group_drives (trip_group_id, drive_id, order_index)
        SELECT new_group, d.id, ROW_NUMBER() OVER (ORDER BY d.start_time) - 1
        FROM drives d
        WHERE d.id IN (SELECT l.drive_id FROM carpool_legs l WHERE l.carpool_trip_id = c.id AND l.drive_id IS NOT NULL)
          AND d.deleted_upstream_at IS NULL;

        UPDATE carpool_trips SET trip_group_id = new_group WHERE id = c.id;
    END LOOP;
END $$;
