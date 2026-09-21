-- ============================================================================
-- TeslaCost Merge Duplicate Trip Groups Migration (Up)
-- Database: PostgreSQL 14+
-- ============================================================================

-- Trip groups covering exactly the same drives are one trip listed twice (a carpool trip got its own group while
-- a group for the same drives already existed). Keep the oldest group and move the expenses and carpool trips of
-- the others onto it. Groups that only partly overlap are left alone.
DO $$
DECLARE
    dup RECORD;
BEGIN
    FOR dup IN
        WITH signatures AS (
            SELECT tg.id, tg.vehicle_id, tg.created_at,
                   (SELECT string_agg(tgd.drive_id::text, ',' ORDER BY tgd.drive_id) FROM trip_group_drives tgd WHERE tgd.trip_group_id = tg.id) AS sig
            FROM trip_groups tg
        ),
        ranked AS (
            SELECT id, vehicle_id, sig,
                   FIRST_VALUE(id) OVER (PARTITION BY vehicle_id, sig ORDER BY created_at, id) AS keeper_id
            FROM signatures
            WHERE sig IS NOT NULL
        )
        SELECT id AS dup_id, keeper_id FROM ranked WHERE id <> keeper_id
    LOOP
        UPDATE drive_expenses SET trip_group_id = dup.keeper_id WHERE trip_group_id = dup.dup_id;
        UPDATE carpool_trips SET trip_group_id = dup.keeper_id WHERE trip_group_id = dup.dup_id;
        DELETE FROM trip_groups WHERE id = dup.dup_id;
    END LOOP;
END $$;
