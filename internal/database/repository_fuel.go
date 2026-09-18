package database

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

// Fuel fill-ups of combustion vehicles (manual entries).

const fuelColumns = `id, vehicle_id, date, odometer, amount, liters, price_per_liter, fuel_type, is_full_tank, notes, created_at, updated_at`

func scanFuelLog(row pgx.Row) (*models.FuelLog, error) {
	var f models.FuelLog
	if err := row.Scan(&f.ID, &f.VehicleID, &f.Date, &f.Odometer, &f.Amount, &f.Liters, &f.PricePerLiter,
		&f.FuelType, &f.IsFullTank, &f.Notes, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return nil, err
	}
	return &f, nil
}

// ListFuelLogs lists the fill-ups of a vehicle, oldest first (date, then odometer).
func (r *Repository) ListFuelLogs(ctx context.Context, vehicleID string) ([]models.FuelLog, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+fuelColumns+` FROM fuel_logs WHERE vehicle_id = $1 ORDER BY date ASC, odometer ASC;`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []models.FuelLog{}
	for rows.Next() {
		f, err := scanFuelLog(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *f)
	}
	return list, rows.Err()
}

// syncOdometerFromFuelLogs raises vehicles.current_odometer to the highest fill-up odometer.
// It never lowers it: the value may come from another source (odometer edit, TeslaMate).
func syncOdometerFromFuelLogs(ctx context.Context, tx pgx.Tx, vehicleID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE vehicles
		SET current_odometer = GREATEST(current_odometer, COALESCE((SELECT MAX(odometer) FROM fuel_logs WHERE vehicle_id = $1), 0)),
		    updated_at = NOW()
		WHERE id = $1;
	`, vehicleID)
	return err
}

// CreateFuelLog stores a fill-up and keeps the vehicle's current odometer up to date.
func (r *Repository) CreateFuelLog(ctx context.Context, f *models.FuelLog) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := tx.QueryRow(ctx, `
		INSERT INTO fuel_logs (vehicle_id, date, odometer, amount, liters, price_per_liter, fuel_type, is_full_tank, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at;
	`, f.VehicleID, f.Date, f.Odometer, f.Amount, f.Liters, f.PricePerLiter, f.FuelType, f.IsFullTank, f.Notes,
	).Scan(&f.ID, &f.CreatedAt, &f.UpdatedAt); err != nil {
		return err
	}
	if err := syncOdometerFromFuelLogs(ctx, tx, f.VehicleID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// UpdateFuelLog replaces the editable fields of a fill-up.
func (r *Repository) UpdateFuelLog(ctx context.Context, f *models.FuelLog) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		UPDATE fuel_logs
		SET date = $3, odometer = $4, amount = $5, liters = $6, price_per_liter = $7,
		    fuel_type = $8, is_full_tank = $9, notes = $10, updated_at = NOW()
		WHERE id::text = $1 AND vehicle_id = $2
		RETURNING created_at, updated_at;
	`, f.ID, f.VehicleID, f.Date, f.Odometer, f.Amount, f.Liters, f.PricePerLiter, f.FuelType, f.IsFullTank, f.Notes,
	).Scan(&f.CreatedAt, &f.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if err := syncOdometerFromFuelLogs(ctx, tx, f.VehicleID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// DeleteFuelLog deletes a fill-up. The vehicle's current odometer is left untouched.
func (r *Repository) DeleteFuelLog(ctx context.Context, vehicleID, fuelLogID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM fuel_logs WHERE id::text = $1 AND vehicle_id = $2;`, fuelLogID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
