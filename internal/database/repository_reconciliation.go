package database

import (
	"context"
	"fmt"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

// TeslaMate upstream-deletion reconciliation and data-quality diagnostics.
// ReconcileTeslaMateRecords flags drives or charges ("drives" | "charges") imported from TeslaMate that were not
// returned by the API over the covered window (start date strictly after coveredAfter, or the whole history when nil).
func (r *Repository) ReconcileTeslaMateRecords(ctx context.Context, resource, vehicleID string, coveredAfter *time.Time, seenIDs []int) (*ReconcileResult, error) {
	var table, idColumn, dateColumn string
	switch resource {
	case "drives":
		table, idColumn, dateColumn = "drives", "teslamate_drive_id", "start_time"
	case "charges":
		table, idColumn, dateColumn = "charge_logs", "teslamate_charge_id", "date"
	default:
		return nil, fmt.Errorf("unknown resource %q", resource)
	}
	if seenIDs == nil {
		seenIDs = []int{}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	window := `vehicle_id = $1 AND ` + idColumn + ` IS NOT NULL AND deleted_upstream_at IS NULL
		AND ($2::timestamptz IS NULL OR ` + dateColumn + ` > $2)`

	var total, missing int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE NOT (`+idColumn+` = ANY($3::int[])))
		FROM `+table+` WHERE `+window+`;
	`, vehicleID, coveredAfter, seenIDs).Scan(&total, &missing); err != nil {
		return nil, fmt.Errorf("failed to count missing %s: %w", resource, err)
	}

	res := &ReconcileResult{Missing: missing}
	if missing == 0 {
		return res, nil
	}
	if missing > 10 && float64(missing) > maxUpstreamDeletionShare*float64(total) {
		res.Skipped = true
		return res, nil
	}

	tag, err := tx.Exec(ctx, `
		UPDATE `+table+` SET deleted_upstream_at = NOW()
		WHERE `+window+` AND NOT (`+idColumn+` = ANY($3::int[]));
	`, vehicleID, coveredAfter, seenIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to flag deleted %s: %w", resource, err)
	}
	res.Marked = int(tag.RowsAffected())
	return res, tx.Commit(ctx)
}

// ============================================================================
// Data Quality
// ============================================================================

// Data quality issue types.
const (
	IssueOdometerGap        = "ODOMETER_GAP"        // Kilometers between two consecutive drives not covered by any drive
	IssueOdometerRegression = "ODOMETER_REGRESSION" // A drive starts below the end odometer of the previous one
	IssueDistanceMismatch   = "DISTANCE_MISMATCH"   // Drive distance differs from its odometer delta
)

// ListDataQualityIssues checks odometer continuity of the drives of a vehicle (most recent first).
func (r *Repository) ListDataQualityIssues(ctx context.Context, vehicleID string, limit int) ([]models.DataQualityIssue, error) {
	rows, err := r.pool.Query(ctx, `
		WITH ordered AS (
			SELECT id, start_time, start_odometer, end_odometer, distance_km,
			       LAG(id) OVER w AS prev_id,
			       LAG(end_odometer) OVER w AS prev_end_odometer
			FROM drives
			WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL
			  AND start_odometer > 0 AND end_odometer > 0
			WINDOW w AS (ORDER BY start_time)
		),
		issues AS (
			SELECT '`+IssueOdometerGap+`' AS type, id, prev_id, start_time, start_odometer - prev_end_odometer AS km
			FROM ordered WHERE start_odometer - prev_end_odometer > 1
			UNION ALL
			SELECT '`+IssueOdometerRegression+`', id, prev_id, start_time, prev_end_odometer - start_odometer
			FROM ordered WHERE prev_end_odometer - start_odometer > 1
			UNION ALL
			SELECT '`+IssueDistanceMismatch+`', id, NULL, start_time, distance_km - (end_odometer - start_odometer)
			FROM ordered
			WHERE ABS(distance_km - (end_odometer - start_odometer)) > GREATEST(1, 0.05 * distance_km)
		)
		SELECT type, id::text, prev_id::text, start_time, ROUND(km::numeric, 1)::float8
		FROM issues
		ORDER BY start_time DESC
		LIMIT $2;
	`, vehicleID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []models.DataQualityIssue{}
	for rows.Next() {
		var issue models.DataQualityIssue
		if err := rows.Scan(&issue.Type, &issue.DriveID, &issue.PreviousDriveID, &issue.Date, &issue.Km); err != nil {
			return nil, err
		}
		list = append(list, issue)
	}
	return list, rows.Err()
}

// OdometerContinuitySummarySQL returns, for vehicle $1, the number of odometer gaps between consecutive drives,
// the kilometers they represent and the number of odometer anomalies (regressions, distance mismatches).
const OdometerContinuitySummarySQL = `
	WITH ordered AS (
		SELECT start_odometer, end_odometer, distance_km,
		       LAG(end_odometer) OVER (ORDER BY start_time) AS prev_end_odometer
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND start_odometer > 0 AND end_odometer > 0
	)
	SELECT COUNT(*) FILTER (WHERE start_odometer - prev_end_odometer > 1),
	       COALESCE(SUM(start_odometer - prev_end_odometer) FILTER (WHERE start_odometer - prev_end_odometer > 1), 0)::float8,
	       COUNT(*) FILTER (WHERE prev_end_odometer - start_odometer > 1
	                           OR ABS(distance_km - (end_odometer - start_odometer)) > GREATEST(1, 0.05 * distance_km))
	FROM ordered;
`

// DataQualitySummary counts odometer continuity issues and the kilometers they represent.
func (r *Repository) DataQualitySummary(ctx context.Context, vehicleID string) (gaps int, gapKm float64, anomalies int, err error) {
	err = r.pool.QueryRow(ctx, OdometerContinuitySummarySQL, vehicleID).Scan(&gaps, &gapKm, &anomalies)
	return gaps, gapKm, anomalies, err
}

// ============================================================================
// Idempotency Keys
// ============================================================================

// StoredResponse is the replayable response of an idempotent request.
type StoredResponse struct {
	Method     string
	Path       string
	StatusCode int
	Body       []byte
}
