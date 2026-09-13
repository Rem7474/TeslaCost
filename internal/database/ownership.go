package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ErrForeignReference is returned when a referenced record does not belong to the target vehicle.
var ErrForeignReference = errors.New("referenced record does not belong to this vehicle")

// ValidationError reports a business rule violation whose message can be shown to the user.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func validationErrorf(format string, args ...any) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

// queryRower is satisfied by both *pgxpool.Pool and pgx.Tx.
type queryRower interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func uniqueStrings(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// ensureDrivesOwned verifies that every drive ID belongs to the vehicle.
func ensureDrivesOwned(ctx context.Context, q queryRower, vehicleID string, driveIDs []string) error {
	ids := uniqueStrings(driveIDs)
	if len(ids) == 0 {
		return nil
	}
	var count int
	if err := q.QueryRow(ctx, `
		SELECT COUNT(*) FROM drives WHERE vehicle_id = $1 AND id::text = ANY($2::text[]);
	`, vehicleID, ids).Scan(&count); err != nil {
		return fmt.Errorf("failed to verify drives ownership: %w", err)
	}
	if count != len(ids) {
		return ErrForeignReference
	}
	return nil
}

// ensureTripGroupOwned verifies that the trip group belongs to the vehicle.
func ensureTripGroupOwned(ctx context.Context, q queryRower, vehicleID, tripGroupID string) error {
	var exists bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM trip_groups WHERE vehicle_id = $1 AND id::text = $2);
	`, vehicleID, tripGroupID).Scan(&exists); err != nil {
		return fmt.Errorf("failed to verify trip group ownership: %w", err)
	}
	if !exists {
		return ErrForeignReference
	}
	return nil
}

// ensureTiresOwned verifies that every tire ID belongs to the vehicle.
func ensureTiresOwned(ctx context.Context, q queryRower, vehicleID string, tireIDs []string) error {
	ids := uniqueStrings(tireIDs)
	if len(ids) == 0 {
		return nil
	}
	var count int
	if err := q.QueryRow(ctx, `
		SELECT COUNT(*) FROM tires WHERE vehicle_id = $1 AND id::text = ANY($2::text[]);
	`, vehicleID, ids).Scan(&count); err != nil {
		return fmt.Errorf("failed to verify tires ownership: %w", err)
	}
	if count != len(ids) {
		return ErrForeignReference
	}
	return nil
}

// EnsureDriveLinksOwned validates optional drive / trip group references of a vehicle-scoped record.
func (r *Repository) EnsureDriveLinksOwned(ctx context.Context, vehicleID string, driveID, tripGroupID *string) error {
	if driveID != nil && *driveID != "" {
		if err := ensureDrivesOwned(ctx, r.pool, vehicleID, []string{*driveID}); err != nil {
			return err
		}
	}
	if tripGroupID != nil && *tripGroupID != "" {
		if err := ensureTripGroupOwned(ctx, r.pool, vehicleID, *tripGroupID); err != nil {
			return err
		}
	}
	return nil
}

// EnsureTireOwned verifies that a tire belongs to the vehicle.
func (r *Repository) EnsureTireOwned(ctx context.Context, vehicleID, tireID string) error {
	err := ensureTiresOwned(ctx, r.pool, vehicleID, []string{tireID})
	if errors.Is(err, ErrForeignReference) {
		return ErrNotFound
	}
	return err
}
