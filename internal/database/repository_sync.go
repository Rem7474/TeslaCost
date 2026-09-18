package database

import (
	"context"
)

// Per-resource sync-state bookkeeping (has a full import completed, last success).
// IsFullImportCompleted reports whether the complete TeslaMate history of a resource was imported once.
func (r *Repository) IsFullImportCompleted(ctx context.Context, vehicleID, resource string) (bool, error) {
	var completed bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM sync_state
			WHERE vehicle_id = $1 AND resource = $2 AND full_import_completed_at IS NOT NULL
		);
	`, vehicleID, resource).Scan(&completed)
	return completed, err
}

// MarkSyncSuccess records a successful sync pass; fullImport marks the complete history as imported.
func (r *Repository) MarkSyncSuccess(ctx context.Context, vehicleID, resource string, fullImport bool) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO sync_state (vehicle_id, resource, full_import_completed_at, last_success_at)
		VALUES ($1, $2, CASE WHEN $3 THEN NOW() END, NOW())
		ON CONFLICT (vehicle_id, resource) DO UPDATE
		SET last_success_at = NOW(),
		    full_import_completed_at = COALESCE(sync_state.full_import_completed_at, EXCLUDED.full_import_completed_at);
	`, vehicleID, resource, fullImport)
	return err
}

// ============================================================================
// Carpooling / BlaBlaCar Module
// ============================================================================
