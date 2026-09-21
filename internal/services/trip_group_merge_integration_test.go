package services

import (
	"context"
	"testing"
	"time"

	"github.com/teslacost/teslacost/migrations"
)

func TestIntegrationMergeDuplicateTripGroups(t *testing.T) {
	db, repo := setupIntegrationDB(t, false)
	ctx := context.Background()
	v := mustVehicle(t, repo, "merge@example.com")
	base := time.Date(2026, 9, 13, 16, 0, 0, 0, time.UTC)
	d1 := mustDrive(t, repo, v.ID, 1, base, 1000, 80)
	d2 := mustDrive(t, repo, v.ID, 2, base.Add(2*time.Hour), 1080, 70)
	d3 := mustDrive(t, repo, v.ID, 3, base.Add(4*time.Hour), 1150, 60)

	group := func(name string, created time.Time, driveIDs ...string) string {
		var id string
		if err := db.Pool.QueryRow(ctx, `INSERT INTO trip_groups (vehicle_id, name, created_at) VALUES ($1, $2, $3) RETURNING id::text`, v.ID, name, created).Scan(&id); err != nil {
			t.Fatal(err)
		}
		for i, d := range driveIDs {
			if _, err := db.Pool.Exec(ctx, `INSERT INTO trip_group_drives (trip_group_id, drive_id, order_index) VALUES ($1, $2, $3)`, id, d, i); err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	oldest := group("oldest", base, d1.ID, d2.ID)
	duplicate := group("duplicate", base.Add(24*time.Hour), d2.ID, d1.ID)
	partial := group("partial", base.Add(24*time.Hour), d1.ID, d2.ID, d3.ID)

	var expenseID string
	if err := db.Pool.QueryRow(ctx, `INSERT INTO drive_expenses (vehicle_id, type, amount, date, trip_group_id) VALUES ($1, 'TOLL', 10, $2, $3) RETURNING id::text`, v.ID, base, duplicate).Scan(&expenseID); err != nil {
		t.Fatal(err)
	}
	var carpoolID string
	if err := db.Pool.QueryRow(ctx, `INSERT INTO carpool_trips (vehicle_id, title, date, trip_group_id) VALUES ($1, 'cp', $2, $3) RETURNING id::text`, v.ID, base, duplicate).Scan(&carpoolID); err != nil {
		t.Fatal(err)
	}

	up, err := migrations.FS.ReadFile("000030_merge_duplicate_trip_groups.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	for run := 1; run <= 2; run++ { // re-runnable: the second run finds nothing to merge
		if _, err := db.Pool.Exec(ctx, string(up)); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
	}

	var remaining []string
	rows, err := db.Pool.Query(ctx, `SELECT id::text FROM trip_groups WHERE vehicle_id = $1`, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		remaining = append(remaining, id)
	}
	rows.Close()
	if len(remaining) != 2 || (remaining[0] != oldest && remaining[1] != oldest) || (remaining[0] != partial && remaining[1] != partial) {
		t.Fatalf("the oldest of two identical groups and the partly overlapping one are kept, got %v", remaining)
	}
	var expenseGroup, carpoolGroup string
	if err := db.Pool.QueryRow(ctx, `SELECT trip_group_id::text FROM drive_expenses WHERE id = $1`, expenseID).Scan(&expenseGroup); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT trip_group_id::text FROM carpool_trips WHERE id = $1`, carpoolID).Scan(&carpoolGroup); err != nil {
		t.Fatal(err)
	}
	if expenseGroup != oldest || carpoolGroup != oldest {
		t.Fatalf("the expense and the carpool follow the kept group, got %s and %s", expenseGroup, carpoolGroup)
	}
}
