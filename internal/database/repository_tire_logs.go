package database

import (
	"context"

	"github.com/teslacost/teslacost/internal/models"
)

// UpdateTireLog corrects a tread depth measurement.
func (r *Repository) UpdateTireLog(ctx context.Context, vehicleID string, l *models.TireLog) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE tire_logs l SET date = $1, odometer = $2, depth_mm = $3, notes = $4
		FROM tires t
		WHERE l.id::text = $5 AND l.tire_id = t.id AND t.id::text = $6 AND t.vehicle_id = $7;
	`, l.Date, l.Odometer, l.DepthMm, l.Notes, l.ID, l.TireID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteTireLog deletes a tread depth measurement.
func (r *Repository) DeleteTireLog(ctx context.Context, vehicleID, tireID, logID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM tire_logs l USING tires t
		WHERE l.id::text = $1 AND l.tire_id = t.id AND t.id::text = $2 AND t.vehicle_id = $3;
	`, logID, tireID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) AddTireLog(ctx context.Context, l *models.TireLog) error {
	query := `
		INSERT INTO tire_logs (tire_id, date, odometer, depth_mm, notes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at;
	`
	return r.pool.QueryRow(ctx, query, l.TireID, l.Date, l.Odometer, l.DepthMm, l.Notes).Scan(&l.ID, &l.CreatedAt)
}

func (r *Repository) ListTireLogs(ctx context.Context, tireID string) ([]models.TireLog, error) {
	query := `
		SELECT id, tire_id, date, odometer, depth_mm, notes, created_at
		FROM tire_logs
		WHERE tire_id::text = $1
		ORDER BY date DESC;
	`
	rows, err := r.pool.Query(ctx, query, tireID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.TireLog
	for rows.Next() {
		var l models.TireLog
		if err := rows.Scan(&l.ID, &l.TireID, &l.Date, &l.Odometer, &l.DepthMm, &l.Notes, &l.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, l)
	}
	return list, rows.Err()
}

// ============================================================================
// Maintenance Expenses
// ============================================================================
