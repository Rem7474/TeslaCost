package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/models"
)

var (
	ErrNotFound = errors.New("record not found")
)

// Repository encapsulates database operations.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new Repository instance.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// ============================================================================
// Users
// ============================================================================

func (r *Repository) CreateUser(ctx context.Context, email, passwordHash string) (*models.User, error) {
	query := `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash, created_at, updated_at;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, email, passwordHash).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, created_at, updated_at
		FROM users
		WHERE email = $1;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, created_at, updated_at
		FROM users
		WHERE id = $1;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return &u, nil
}

// ============================================================================
// Vehicles
// ============================================================================

func (r *Repository) CreateVehicle(ctx context.Context, v *models.Vehicle) error {
	query := `
		INSERT INTO vehicles (
			user_id, name, vin, teslamate_car_id, current_odometer,
			teslamate_api_url, teslamate_auth_type, teslamate_api_key_encrypted,
			teslamate_basic_user, teslamate_basic_pass_encrypted
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at;
	`
	err := r.pool.QueryRow(ctx, query,
		v.UserID, v.Name, v.Vin, v.TeslaMateCarID, v.CurrentOdometer,
		v.TeslaMateAPIURL, v.TeslaMateAuthType, v.TeslaMateAPIKeyEncrypted,
		v.TeslaMateBasicUser, v.TeslaMateBasicPassEnc,
	).Scan(&v.ID, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create vehicle: %w", err)
	}
	return nil
}

func (r *Repository) ListVehiclesByUserID(ctx context.Context, userID string) ([]models.Vehicle, error) {
	query := `
		SELECT id, user_id, name, vin, teslamate_car_id, current_odometer,
		       teslamate_api_url, teslamate_auth_type, teslamate_api_key_encrypted,
		       teslamate_basic_user, teslamate_basic_pass_encrypted, created_at, updated_at
		FROM vehicles
		WHERE user_id = $1
		ORDER BY created_at ASC;
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list vehicles: %w", err)
	}
	defer rows.Close()

	var list []models.Vehicle
	for rows.Next() {
		var v models.Vehicle
		if err := rows.Scan(
			&v.ID, &v.UserID, &v.Name, &v.Vin, &v.TeslaMateCarID, &v.CurrentOdometer,
			&v.TeslaMateAPIURL, &v.TeslaMateAuthType, &v.TeslaMateAPIKeyEncrypted,
			&v.TeslaMateBasicUser, &v.TeslaMateBasicPassEnc, &v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, v)
	}
	return list, nil
}

func (r *Repository) GetVehicleByID(ctx context.Context, id, userID string) (*models.Vehicle, error) {
	query := `
		SELECT id, user_id, name, vin, teslamate_car_id, current_odometer,
		       teslamate_api_url, teslamate_auth_type, teslamate_api_key_encrypted,
		       teslamate_basic_user, teslamate_basic_pass_encrypted, created_at, updated_at
		FROM vehicles
		WHERE id = $1 AND user_id = $2;
	`
	var v models.Vehicle
	err := r.pool.QueryRow(ctx, query, id, userID).Scan(
		&v.ID, &v.UserID, &v.Name, &v.Vin, &v.TeslaMateCarID, &v.CurrentOdometer,
		&v.TeslaMateAPIURL, &v.TeslaMateAuthType, &v.TeslaMateAPIKeyEncrypted,
		&v.TeslaMateBasicUser, &v.TeslaMateBasicPassEnc, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get vehicle: %w", err)
	}
	return &v, nil
}

func (r *Repository) UpdateVehicle(ctx context.Context, v *models.Vehicle) error {
	query := `
		UPDATE vehicles
		SET name = $1, vin = $2, teslamate_car_id = $3, current_odometer = $4,
		    teslamate_api_url = $5, teslamate_auth_type = $6,
		    teslamate_api_key_encrypted = $7, teslamate_basic_user = $8,
		    teslamate_basic_pass_encrypted = $9, updated_at = NOW()
		WHERE id = $10 AND user_id = $11;
	`
	tag, err := r.pool.Exec(ctx, query,
		v.Name, v.Vin, v.TeslaMateCarID, v.CurrentOdometer,
		v.TeslaMateAPIURL, v.TeslaMateAuthType, v.TeslaMateAPIKeyEncrypted,
		v.TeslaMateBasicUser, v.TeslaMateBasicPassEnc, v.ID, v.UserID,
	)
	if err != nil {
		return fmt.Errorf("failed to update vehicle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateVehicleOdometer(ctx context.Context, vehicleID string, odometer float64) error {
	query := `
		UPDATE vehicles
		SET current_odometer = $1, updated_at = NOW()
		WHERE id = $2;
	`
	_, err := r.pool.Exec(ctx, query, odometer, vehicleID)
	return err
}

func (r *Repository) DeleteVehicle(ctx context.Context, id, userID string) error {
	query := `DELETE FROM vehicles WHERE id = $1 AND user_id = $2;`
	tag, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete vehicle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Drives & Trip Groups
// ============================================================================

func (r *Repository) UpsertTeslaMateDrive(ctx context.Context, d *models.Drive) error {
	query := `
		INSERT INTO drives (
			vehicle_id, teslamate_drive_id, start_time, end_time,
			start_odometer, end_odometer, distance_km, duration_min,
			speed_avg, start_address, end_address, energy_consumed_kwh,
			consumption_kwh_100km, tags, is_manual
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, FALSE)
		ON CONFLICT (vehicle_id, teslamate_drive_id) DO UPDATE
		SET start_time = EXCLUDED.start_time,
		    end_time = EXCLUDED.end_time,
		    start_odometer = EXCLUDED.start_odometer,
		    end_odometer = EXCLUDED.end_odometer,
		    distance_km = EXCLUDED.distance_km,
		    duration_min = EXCLUDED.duration_min,
		    speed_avg = EXCLUDED.speed_avg,
		    start_address = EXCLUDED.start_address,
		    end_address = EXCLUDED.end_address,
		    energy_consumed_kwh = EXCLUDED.energy_consumed_kwh,
		    consumption_kwh_100km = EXCLUDED.consumption_kwh_100km,
		    updated_at = NOW()
		RETURNING id;
	`
	return r.pool.QueryRow(ctx, query,
		d.VehicleID, d.TeslaMateDriveID, d.StartTime, d.EndTime,
		d.StartOdometer, d.EndOdometer, d.DistanceKm, d.DurationMin,
		d.SpeedAvg, d.StartAddress, d.EndAddress, d.EnergyConsumedKwh,
		d.ConsumptionKwh100km, d.Tags,
	).Scan(&d.ID)
}

func (r *Repository) ListDrives(ctx context.Context, vehicleID string, tag string, limit, offset int) ([]models.Drive, int, error) {
	countQuery := `
		SELECT COUNT(*)
		FROM drives
		WHERE vehicle_id = $1 AND ($2 = '' OR $2 = ANY(tags));
	`
	var total int
	if err := r.pool.QueryRow(ctx, countQuery, vehicleID, tag).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, vehicle_id, teslamate_drive_id, start_time, end_time,
		       start_odometer, end_odometer, distance_km, duration_min,
		       speed_avg, start_address, end_address, energy_consumed_kwh,
		       consumption_kwh_100km, tags, is_manual, created_at, updated_at
		FROM drives
		WHERE vehicle_id = $1 AND ($2 = '' OR $2 = ANY(tags))
		ORDER BY start_time DESC
		LIMIT $3 OFFSET $4;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID, tag, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []models.Drive
	for rows.Next() {
		var d models.Drive
		if err := rows.Scan(
			&d.ID, &d.VehicleID, &d.TeslaMateDriveID, &d.StartTime, &d.EndTime,
			&d.StartOdometer, &d.EndOdometer, &d.DistanceKm, &d.DurationMin,
			&d.SpeedAvg, &d.StartAddress, &d.EndAddress, &d.EnergyConsumedKwh,
			&d.ConsumptionKwh100km, &d.Tags, &d.IsManual, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, d)
	}
	return list, total, nil
}

func (r *Repository) UpdateDriveTags(ctx context.Context, driveID, vehicleID string, tags []string) error {
	query := `
		UPDATE drives
		SET tags = $1, updated_at = NOW()
		WHERE id = $2 AND vehicle_id = $3;
	`
	tag, err := r.pool.Exec(ctx, query, tags, driveID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CreateTripGroup(ctx context.Context, tg *models.TripGroup, driveIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO trip_groups (vehicle_id, name, notes)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at;
	`
	if err := tx.QueryRow(ctx, query, tg.VehicleID, tg.Name, tg.Notes).Scan(&tg.ID, &tg.CreatedAt, &tg.UpdatedAt); err != nil {
		return err
	}

	for i, dID := range driveIDs {
		_, err := tx.Exec(ctx, `
			INSERT INTO trip_group_drives (trip_group_id, drive_id, order_index)
			VALUES ($1, $2, $3);
		`, tg.ID, dID, i)
		if err != nil {
			return fmt.Errorf("failed to link drive to trip group: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) ListTripGroups(ctx context.Context, vehicleID string) ([]models.TripGroup, error) {
	query := `
		SELECT id, vehicle_id, name, notes, created_at, updated_at
		FROM trip_groups
		WHERE vehicle_id = $1
		ORDER BY created_at DESC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.TripGroup
	for rows.Next() {
		var tg models.TripGroup
		if err := rows.Scan(&tg.ID, &tg.VehicleID, &tg.Name, &tg.Notes, &tg.CreatedAt, &tg.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, tg)
	}
	return list, nil
}

// ============================================================================
// Drive Expenses (Tolls, Parking)
// ============================================================================

func (r *Repository) CreateDriveExpense(ctx context.Context, exp *models.DriveExpense) error {
	query := `
		INSERT INTO drive_expenses (vehicle_id, trip_group_id, drive_id, type, amount, currency, date, notes)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at;
	`
	return r.pool.QueryRow(ctx, query,
		exp.VehicleID, exp.TripGroupID, exp.DriveID, exp.Type,
		exp.Amount, exp.Currency, exp.Date, exp.Notes,
	).Scan(&exp.ID, &exp.CreatedAt)
}

func (r *Repository) ListDriveExpenses(ctx context.Context, vehicleID string) ([]models.DriveExpense, error) {
	query := `
		SELECT id, vehicle_id, trip_group_id, drive_id, type, amount, currency, date, notes, created_at
		FROM drive_expenses
		WHERE vehicle_id = $1
		ORDER BY date DESC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.DriveExpense
	for rows.Next() {
		var e models.DriveExpense
		if err := rows.Scan(
			&e.ID, &e.VehicleID, &e.TripGroupID, &e.DriveID, &e.Type,
			&e.Amount, &e.Currency, &e.Date, &e.Notes, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, nil
}

// ============================================================================
// Tires, Wear Logs & Rotations
// ============================================================================

func (r *Repository) CreateTire(ctx context.Context, t *models.Tire) error {
	query := `
		INSERT INTO tires (
			vehicle_id, brand, model, dimension, season,
			purchase_date, purchase_price, current_position,
			initial_depth_mm, min_legal_depth_mm, dot_code
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at;
	`
	return r.pool.QueryRow(ctx, query,
		t.VehicleID, t.Brand, t.Model, t.Dimension, t.Season,
		t.PurchaseDate, t.PurchasePrice, t.CurrentPosition,
		t.InitialDepthMm, t.MinLegalDepthMm, t.DotCode,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
}

func (r *Repository) ListTires(ctx context.Context, vehicleID string) ([]models.Tire, error) {
	query := `
		SELECT id, vehicle_id, brand, model, dimension, season,
		       purchase_date, purchase_price, current_position,
		       initial_depth_mm, min_legal_depth_mm, dot_code, is_archived,
		       created_at, updated_at
		FROM tires
		WHERE vehicle_id = $1 AND is_archived = FALSE
		ORDER BY current_position ASC, purchase_date DESC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Tire
	for rows.Next() {
		var t models.Tire
		if err := rows.Scan(
			&t.ID, &t.VehicleID, &t.Brand, &t.Model, &t.Dimension, &t.Season,
			&t.PurchaseDate, &t.PurchasePrice, &t.CurrentPosition,
			&t.InitialDepthMm, &t.MinLegalDepthMm, &t.DotCode, &t.IsArchived,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

func (r *Repository) UpdateTirePosition(ctx context.Context, tireID string, pos models.TirePosition) error {
	query := `UPDATE tires SET current_position = $1, updated_at = NOW() WHERE id = $2;`
	_, err := r.pool.Exec(ctx, query, pos, tireID)
	return err
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
		WHERE tire_id = $1
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
	return list, nil
}

func (r *Repository) AddTireRotation(ctx context.Context, rot *models.TireRotation) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	mappingBytes, err := json.Marshal(rot.MappingJSON)
	if err != nil {
		return fmt.Errorf("invalid mapping JSON: %w", err)
	}

	query := `
		INSERT INTO tire_rotations (vehicle_id, date, odometer, mapping_json, notes)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at;
	`
	if err := tx.QueryRow(ctx, query, rot.VehicleID, rot.Date, rot.Odometer, mappingBytes, rot.Notes).Scan(&rot.ID, &rot.CreatedAt); err != nil {
		return err
	}

	// Update positions for each tire mapped
	for posStr, tireIDVal := range rot.MappingJSON {
		if tireID, ok := tireIDVal.(string); ok && tireID != "" {
			_, err := tx.Exec(ctx, `UPDATE tires SET current_position = $1, updated_at = NOW() WHERE id = $2;`, posStr, tireID)
			if err != nil {
				return fmt.Errorf("failed to update tire position during rotation: %w", err)
			}
		}
	}

	return tx.Commit(ctx)
}

// ============================================================================
// Maintenance Expenses
// ============================================================================

func (r *Repository) CreateMaintenanceExpense(ctx context.Context, m *models.MaintenanceExpense) error {
	query := `
		INSERT INTO maintenance_expenses (
			vehicle_id, category, amount, currency, date,
			odometer, is_recurring, recurrence_interval_months, description
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at;
	`
	return r.pool.QueryRow(ctx, query,
		m.VehicleID, m.Category, m.Amount, m.Currency, m.Date,
		m.Odometer, m.IsRecurring, m.RecurrenceIntervalMonths, m.Description,
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
}

func (r *Repository) ListMaintenanceExpenses(ctx context.Context, vehicleID string) ([]models.MaintenanceExpense, error) {
	query := `
		SELECT id, vehicle_id, category, amount, currency, date,
		       odometer, is_recurring, recurrence_interval_months, description,
		       created_at, updated_at
		FROM maintenance_expenses
		WHERE vehicle_id = $1
		ORDER BY date DESC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.MaintenanceExpense
	for rows.Next() {
		var m models.MaintenanceExpense
		if err := rows.Scan(
			&m.ID, &m.VehicleID, &m.Category, &m.Amount, &m.Currency, &m.Date,
			&m.Odometer, &m.IsRecurring, &m.RecurrenceIntervalMonths, &m.Description,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, nil
}

// ============================================================================
// Charges
// ============================================================================

func (r *Repository) UpsertTeslaMateCharge(ctx context.Context, c *models.ChargeLog) error {
	query := `
		INSERT INTO charge_logs (
			vehicle_id, teslamate_charge_id, date, end_date,
			address, kwh_added, kwh_used, cost, currency, odometer, is_manual
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, FALSE)
		ON CONFLICT (vehicle_id, teslamate_charge_id) DO UPDATE
		SET date = EXCLUDED.date,
		    end_date = EXCLUDED.end_date,
		    address = EXCLUDED.address,
		    kwh_added = EXCLUDED.kwh_added,
		    kwh_used = EXCLUDED.kwh_used,
		    cost = EXCLUDED.cost,
		    currency = EXCLUDED.currency,
		    odometer = EXCLUDED.odometer
		RETURNING id;
	`
	return r.pool.QueryRow(ctx, query,
		c.VehicleID, c.TeslaMateChargeID, c.Date, c.EndDate,
		c.Address, c.KwhAdded, c.KwhUsed, c.Cost, c.Currency, c.Odometer,
	).Scan(&c.ID)
}

func (r *Repository) ListCharges(ctx context.Context, vehicleID string, limit, offset int) ([]models.ChargeLog, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM charge_logs WHERE vehicle_id = $1;`, vehicleID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, vehicle_id, teslamate_charge_id, date, end_date,
		       address, kwh_added, kwh_used, cost, currency, odometer, is_manual, created_at
		FROM charge_logs
		WHERE vehicle_id = $1
		ORDER BY date DESC
		LIMIT $2 OFFSET $3;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []models.ChargeLog
	for rows.Next() {
		var c models.ChargeLog
		if err := rows.Scan(
			&c.ID, &c.VehicleID, &c.TeslaMateChargeID, &c.Date, &c.EndDate,
			&c.Address, &c.KwhAdded, &c.KwhUsed, &c.Cost, &c.Currency, &c.Odometer,
			&c.IsManual, &c.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, c)
	}
	return list, total, nil
}
