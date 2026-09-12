package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

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

func (r *Repository) GetUserCount(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users;`
	var count int
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count users: %w", err)
	}
	return count, nil
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

func (r *Repository) ListAllVehiclesWithTeslaMate(ctx context.Context) ([]models.Vehicle, error) {
	query := `
		SELECT id, user_id, name, vin, teslamate_car_id, current_odometer,
		       teslamate_api_url, teslamate_auth_type, teslamate_api_key_encrypted,
		       teslamate_basic_user, teslamate_basic_pass_encrypted, created_at, updated_at
		FROM vehicles
		WHERE teslamate_api_url IS NOT NULL AND teslamate_api_url != ''
		ORDER BY created_at ASC;
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list vehicles with teslamate: %w", err)
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

func (r *Repository) UpsertTeslaMateDrive(ctx context.Context, d *models.Drive) (bool, error) {
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
		RETURNING id, (xmax = 0) AS is_inserted;
	`
	var isInserted bool
	err := r.pool.QueryRow(ctx, query,
		d.VehicleID, d.TeslaMateDriveID, d.StartTime, d.EndTime,
		d.StartOdometer, d.EndOdometer, d.DistanceKm, d.DurationMin,
		d.SpeedAvg, d.StartAddress, d.EndAddress, d.EnergyConsumedKwh,
		d.ConsumptionKwh100km, d.Tags,
	).Scan(&d.ID, &isInserted)
	return isInserted, err
}

func (r *Repository) GetLatestTeslaMateDriveStartTime(ctx context.Context, vehicleID string) (*time.Time, error) {
	query := `
		SELECT start_time
		FROM drives
		WHERE vehicle_id = $1 AND teslamate_drive_id IS NOT NULL
		ORDER BY start_time DESC
		LIMIT 1;
	`
	var t time.Time
	err := r.pool.QueryRow(ctx, query, vehicleID).Scan(&t)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
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
	lifespan := t.EstimatedLifespanKm
	if lifespan <= 0 {
		lifespan = 40000
	}
	query := `
		INSERT INTO tires (
			vehicle_id, brand, model, dimension, season,
			purchase_date, purchase_price, current_position,
			initial_depth_mm, min_legal_depth_mm, dot_code,
			mounted_odometer, accumulated_distance_km, estimated_lifespan_km
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, created_at, updated_at;
	`
	err := r.pool.QueryRow(ctx, query,
		t.VehicleID, t.Brand, t.Model, t.Dimension, t.Season,
		t.PurchaseDate, t.PurchasePrice, t.CurrentPosition,
		t.InitialDepthMm, t.MinLegalDepthMm, t.DotCode,
		t.MountedOdometer, t.AccumulatedDistanceKm, lifespan,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return err
	}

	// If mounted immediately, create initial mount session
	if t.CurrentPosition != models.TirePosStorage && t.CurrentPosition != models.TirePosDisposed && t.MountedOdometer != nil && t.VehicleID != nil {
		sessionQuery := `
			INSERT INTO tire_mount_sessions (
				tire_id, vehicle_id, position, mounted_date, mounted_odometer
			) VALUES ($1, $2, $3, $4, $5);
		`
		_, _ = r.pool.Exec(ctx, sessionQuery, t.ID, *t.VehicleID, t.CurrentPosition, t.PurchaseDate, *t.MountedOdometer)
	}

	return nil
}

func (r *Repository) ListTires(ctx context.Context, vehicleID string) ([]models.Tire, error) {
	query := `
		SELECT id, vehicle_id, brand, model, dimension, season,
		       purchase_date, purchase_price, current_position,
		       initial_depth_mm, min_legal_depth_mm, dot_code, is_archived,
		       mounted_odometer, accumulated_distance_km, estimated_lifespan_km,
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
			&t.MountedOdometer, &t.AccumulatedDistanceKm, &t.EstimatedLifespanKm,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, nil
}

func (r *Repository) GetTireByID(ctx context.Context, id, vehicleID string) (*models.Tire, error) {
	query := `
		SELECT id, vehicle_id, brand, model, dimension, season,
		       purchase_date, purchase_price, current_position,
		       initial_depth_mm, min_legal_depth_mm, dot_code, is_archived,
		       mounted_odometer, accumulated_distance_km, estimated_lifespan_km,
		       created_at, updated_at
		FROM tires
		WHERE id = $1 AND vehicle_id = $2;
	`
	var t models.Tire
	err := r.pool.QueryRow(ctx, query, id, vehicleID).Scan(
		&t.ID, &t.VehicleID, &t.Brand, &t.Model, &t.Dimension, &t.Season,
		&t.PurchaseDate, &t.PurchasePrice, &t.CurrentPosition,
		&t.InitialDepthMm, &t.MinLegalDepthMm, &t.DotCode, &t.IsArchived,
		&t.MountedOdometer, &t.AccumulatedDistanceKm, &t.EstimatedLifespanKm,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *Repository) UpdateTire(ctx context.Context, t *models.Tire) error {
	lifespan := t.EstimatedLifespanKm
	if lifespan <= 0 {
		lifespan = 40000
	}
	query := `
		UPDATE tires
		SET brand = $1, model = $2, dimension = $3, season = $4,
		    purchase_date = $5, purchase_price = $6, current_position = $7,
		    initial_depth_mm = $8, min_legal_depth_mm = $9, dot_code = $10,
		    mounted_odometer = $11, accumulated_distance_km = $12, estimated_lifespan_km = $13,
		    updated_at = NOW()
		WHERE id = $14;
	`
	_, err := r.pool.Exec(ctx, query,
		t.Brand, t.Model, t.Dimension, t.Season,
		t.PurchaseDate, t.PurchasePrice, t.CurrentPosition,
		t.InitialDepthMm, t.MinLegalDepthMm, t.DotCode,
		t.MountedOdometer, t.AccumulatedDistanceKm, lifespan,
		t.ID,
	)
	return err
}

func (r *Repository) CreateTiresBatch(ctx context.Context, tires []*models.Tire) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	for _, t := range tires {
		lifespan := t.EstimatedLifespanKm
		if lifespan <= 0 {
			lifespan = 40000
		}
		query := `
			INSERT INTO tires (
				vehicle_id, brand, model, dimension, season,
				purchase_date, purchase_price, current_position,
				initial_depth_mm, min_legal_depth_mm, dot_code,
				mounted_odometer, accumulated_distance_km, estimated_lifespan_km
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			RETURNING id, created_at, updated_at;
		`
		if err := tx.QueryRow(ctx, query,
			t.VehicleID, t.Brand, t.Model, t.Dimension, t.Season,
			t.PurchaseDate, t.PurchasePrice, t.CurrentPosition,
			t.InitialDepthMm, t.MinLegalDepthMm, t.DotCode,
			t.MountedOdometer, t.AccumulatedDistanceKm, lifespan,
		).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return fmt.Errorf("failed to batch insert tire: %w", err)
		}

		if t.CurrentPosition != models.TirePosStorage && t.CurrentPosition != models.TirePosDisposed && t.MountedOdometer != nil && t.VehicleID != nil {
			sessionQuery := `
				INSERT INTO tire_mount_sessions (
					tire_id, vehicle_id, position, mounted_date, mounted_odometer
				) VALUES ($1, $2, $3, $4, $5);
			`
			if _, err := tx.Exec(ctx, sessionQuery, t.ID, *t.VehicleID, t.CurrentPosition, t.PurchaseDate, *t.MountedOdometer); err != nil {
				return fmt.Errorf("failed to create mount session: %w", err)
			}
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) ListTireMountSessions(ctx context.Context, tireID string) ([]models.TireMountSession, error) {
	query := `
		SELECT id, tire_id, vehicle_id, position, mounted_date, mounted_odometer,
		       dismounted_date, dismounted_odometer, distance_km, notes, created_at, updated_at
		FROM tire_mount_sessions
		WHERE tire_id = $1
		ORDER BY mounted_date DESC;
	`
	rows, err := r.pool.Query(ctx, query, tireID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.TireMountSession
	for rows.Next() {
		var s models.TireMountSession
		if err := rows.Scan(
			&s.ID, &s.TireID, &s.VehicleID, &s.Position, &s.MountedDate, &s.MountedOdometer,
			&s.DismountedDate, &s.DismountedOdometer, &s.DistanceKm, &s.Notes, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	if list == nil {
		list = []models.TireMountSession{}
	}
	return list, nil
}

func (r *Repository) CreateTireMountSession(ctx context.Context, s *models.TireMountSession) error {
	if s.DistanceKm == 0 && s.DismountedOdometer != nil && *s.DismountedOdometer > s.MountedOdometer {
		s.DistanceKm = *s.DismountedOdometer - s.MountedOdometer
	}
	query := `
		INSERT INTO tire_mount_sessions (
			tire_id, vehicle_id, position, mounted_date, mounted_odometer,
			dismounted_date, dismounted_odometer, distance_km, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at;
	`
	err := r.pool.QueryRow(ctx, query,
		s.TireID, s.VehicleID, s.Position, s.MountedDate, s.MountedOdometer,
		s.DismountedDate, s.DismountedOdometer, s.DistanceKm, s.Notes,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return err
	}
	return r.RecalculateTireLifetimeDistance(ctx, s.TireID)
}

func (r *Repository) UpdateTireMountSession(ctx context.Context, s *models.TireMountSession) error {
	if s.DistanceKm == 0 && s.DismountedOdometer != nil && *s.DismountedOdometer > s.MountedOdometer {
		s.DistanceKm = *s.DismountedOdometer - s.MountedOdometer
	}
	query := `
		UPDATE tire_mount_sessions
		SET position = $1, mounted_date = $2, mounted_odometer = $3,
		    dismounted_date = $4, dismounted_odometer = $5, distance_km = $6, notes = $7,
		    updated_at = NOW()
		WHERE id = $8 AND tire_id = $9;
	`
	_, err := r.pool.Exec(ctx, query,
		s.Position, s.MountedDate, s.MountedOdometer,
		s.DismountedDate, s.DismountedOdometer, s.DistanceKm, s.Notes,
		s.ID, s.TireID,
	)
	if err != nil {
		return err
	}
	return r.RecalculateTireLifetimeDistance(ctx, s.TireID)
}

func (r *Repository) DeleteTireMountSession(ctx context.Context, sessionID, tireID string) error {
	query := `DELETE FROM tire_mount_sessions WHERE id = $1 AND tire_id = $2;`
	cmd, err := r.pool.Exec(ctx, query, sessionID, tireID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return r.RecalculateTireLifetimeDistance(ctx, tireID)
}

func (r *Repository) RecalculateTireLifetimeDistance(ctx context.Context, tireID string) error {
	var totalFinishedKm float64
	_ = r.pool.QueryRow(ctx, `
		SELECT COALESCE(SUM(distance_km), 0)
		FROM tire_mount_sessions
		WHERE tire_id = $1 AND dismounted_date IS NOT NULL;
	`, tireID).Scan(&totalFinishedKm)

	var activeMountedOdometer *float64
	_ = r.pool.QueryRow(ctx, `
		SELECT mounted_odometer
		FROM tire_mount_sessions
		WHERE tire_id = $1 AND dismounted_date IS NULL
		ORDER BY mounted_date DESC LIMIT 1;
	`, tireID).Scan(&activeMountedOdometer)

	_, err := r.pool.Exec(ctx, `
		UPDATE tires
		SET accumulated_distance_km = $1,
		    mounted_odometer = $2,
		    updated_at = NOW()
		WHERE id = $3;
	`, totalFinishedKm, activeMountedOdometer, tireID)
	return err
}

func (r *Repository) QuickRotateTires(ctx context.Context, vehicleID string, mode string, odometer float64, swapWithPackTireIDs []string) error {
	currentTires, err := r.ListTires(ctx, vehicleID)
	if err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	mountedMap := make(map[models.TirePosition]*models.Tire)
	for i := range currentTires {
		t := &currentTires[i]
		if t.CurrentPosition != models.TirePosStorage && t.CurrentPosition != models.TirePosDisposed {
			mountedMap[t.CurrentPosition] = t
		}
	}

	newPositions := make(map[string]models.TirePosition)
	now := time.Now().UTC()

	switch mode {
	case "FRONT_BACK":
		if fl, ok := mountedMap[models.TirePosFL]; ok {
			newPositions[fl.ID] = models.TirePosRL
		}
		if rl, ok := mountedMap[models.TirePosRL]; ok {
			newPositions[rl.ID] = models.TirePosFL
		}
		if fr, ok := mountedMap[models.TirePosFR]; ok {
			newPositions[fr.ID] = models.TirePosRR
		}
		if rr, ok := mountedMap[models.TirePosRR]; ok {
			newPositions[rr.ID] = models.TirePosFR
		}
	case "CROSS":
		if fl, ok := mountedMap[models.TirePosFL]; ok {
			newPositions[fl.ID] = models.TirePosRR
		}
		if rr, ok := mountedMap[models.TirePosRR]; ok {
			newPositions[rr.ID] = models.TirePosFL
		}
		if fr, ok := mountedMap[models.TirePosFR]; ok {
			newPositions[fr.ID] = models.TirePosRL
		}
		if rl, ok := mountedMap[models.TirePosRL]; ok {
			newPositions[rl.ID] = models.TirePosFR
		}
	case "SWAP_PACK":
		for _, t := range mountedMap {
			newPositions[t.ID] = models.TirePosStorage
		}
		positions := []models.TirePosition{models.TirePosFL, models.TirePosFR, models.TirePosRL, models.TirePosRR}
		for i, tireID := range swapWithPackTireIDs {
			if i < len(positions) {
				newPositions[tireID] = positions[i]
			}
		}
	default:
		return fmt.Errorf("unknown rotation mode: %s", mode)
	}

	mappingJSON := make(map[string]any)
	for tireID, newPos := range newPositions {
		mappingJSON[string(newPos)] = tireID

		var currentTire *models.Tire
		for i := range currentTires {
			if currentTires[i].ID == tireID {
				currentTire = &currentTires[i]
				break
			}
		}

		if currentTire != nil {
			if currentTire.CurrentPosition != models.TirePosStorage && newPos == models.TirePosStorage {
				var runKm float64
				if currentTire.MountedOdometer != nil && odometer > *currentTire.MountedOdometer {
					runKm = odometer - *currentTire.MountedOdometer
				}
				_, _ = tx.Exec(ctx, `
					UPDATE tire_mount_sessions
					SET dismounted_date = $1, dismounted_odometer = $2, distance_km = $3, updated_at = NOW()
					WHERE tire_id = $4 AND dismounted_date IS NULL;
				`, now, odometer, runKm, tireID)

				newAcc := currentTire.AccumulatedDistanceKm + runKm
				_, _ = tx.Exec(ctx, `
					UPDATE tires
					SET current_position = $1, mounted_odometer = NULL, accumulated_distance_km = $2, updated_at = NOW()
					WHERE id = $3;
				`, newPos, newAcc, tireID)
			} else if newPos != models.TirePosStorage {
				if currentTire.CurrentPosition != models.TirePosStorage {
					var runKm float64
					if currentTire.MountedOdometer != nil && odometer > *currentTire.MountedOdometer {
						runKm = odometer - *currentTire.MountedOdometer
					}
					_, _ = tx.Exec(ctx, `
						UPDATE tire_mount_sessions
						SET dismounted_date = $1, dismounted_odometer = $2, distance_km = $3, updated_at = NOW()
						WHERE tire_id = $4 AND dismounted_date IS NULL;
					`, now, odometer, runKm, tireID)
					currentTire.AccumulatedDistanceKm += runKm
				}

				_, _ = tx.Exec(ctx, `
					INSERT INTO tire_mount_sessions (
						tire_id, vehicle_id, position, mounted_date, mounted_odometer
					) VALUES ($1, $2, $3, $4, $5);
				`, tireID, vehicleID, newPos, now, odometer)

				_, _ = tx.Exec(ctx, `
					UPDATE tires
					SET current_position = $1, mounted_odometer = $2, accumulated_distance_km = $3, updated_at = NOW()
					WHERE id = $4;
				`, newPos, odometer, currentTire.AccumulatedDistanceKm, tireID)
			}
		}
	}

	mapBytes, _ := json.Marshal(mappingJSON)
	_, _ = tx.Exec(ctx, `
		INSERT INTO tire_rotations (vehicle_id, date, odometer, mapping_json, notes)
		VALUES ($1, $2, $3, $4, $5);
	`, vehicleID, now, odometer, mapBytes, fmt.Sprintf("Permutation rapide: %s", mode))

	return tx.Commit(ctx)
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

func (r *Repository) UpsertTeslaMateCharge(ctx context.Context, c *models.ChargeLog) (bool, error) {
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
		RETURNING id, (xmax = 0) AS is_inserted;
	`
	var isInserted bool
	err := r.pool.QueryRow(ctx, query,
		c.VehicleID, c.TeslaMateChargeID, c.Date, c.EndDate,
		c.Address, c.KwhAdded, c.KwhUsed, c.Cost, c.Currency, c.Odometer,
	).Scan(&c.ID, &isInserted)
	return isInserted, err
}

func (r *Repository) GetLatestTeslaMateChargeDate(ctx context.Context, vehicleID string) (*time.Time, error) {
	query := `
		SELECT date
		FROM charge_logs
		WHERE vehicle_id = $1 AND teslamate_charge_id IS NOT NULL
		ORDER BY date DESC
		LIMIT 1;
	`
	var t time.Time
	err := r.pool.QueryRow(ctx, query, vehicleID).Scan(&t)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
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

// ============================================================================
// Carpooling / BlaBlaCar Module
// ============================================================================

func (r *Repository) CreateCarpoolTrip(ctx context.Context, trip *models.CarpoolTrip, passengers []models.CarpoolPassenger) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Calculate totals
	var revenue float64
	for _, p := range passengers {
		revenue += p.AmountPaid
	}
	trip.TotalRevenue = revenue
	trip.TotalCost = trip.ElectricityCost + trip.TollsCost + trip.TiresCost + trip.MaintenanceCost + trip.InsuranceCost + trip.OtherCost
	trip.NetCost = trip.TotalCost - trip.TotalRevenue

	query := `
		INSERT INTO carpool_trips (
			vehicle_id, drive_id, trip_group_id, title, date, distance_km,
			electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost,
			total_cost, total_revenue, net_cost, notes
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16
		)
		RETURNING id, created_at, updated_at;
	`
	err = tx.QueryRow(ctx, query,
		trip.VehicleID, trip.DriveID, trip.TripGroupID, trip.Title, trip.Date, trip.DistanceKm,
		trip.ElectricityCost, trip.TollsCost, trip.TiresCost, trip.MaintenanceCost, trip.InsuranceCost, trip.OtherCost,
		trip.TotalCost, trip.TotalRevenue, trip.NetCost, trip.Notes,
	).Scan(&trip.ID, &trip.CreatedAt, &trip.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert carpool trip: %w", err)
	}

	for i := range passengers {
		p := &passengers[i]
		p.CarpoolTripID = trip.ID
		pQuery := `
			INSERT INTO carpool_passengers (
				carpool_trip_id, passenger_name, origin, destination, seats, amount_paid, notes
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, created_at;
		`
		if err := tx.QueryRow(ctx, pQuery,
			p.CarpoolTripID, p.PassengerName, p.Origin, p.Destination, p.Seats, p.AmountPaid, p.Notes,
		).Scan(&p.ID, &p.CreatedAt); err != nil {
			return fmt.Errorf("failed to insert carpool passenger: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) ListCarpoolTrips(ctx context.Context, vehicleID string) ([]models.CarpoolTripWithPassengers, error) {
	query := `
		SELECT id, vehicle_id, drive_id, trip_group_id, title, date, distance_km,
		       electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost,
		       total_cost, total_revenue, net_cost, notes, created_at, updated_at
		FROM carpool_trips
		WHERE vehicle_id = $1
		ORDER BY date DESC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("failed to list carpool trips: %w", err)
	}
	defer rows.Close()

	var trips []models.CarpoolTripWithPassengers
	for rows.Next() {
		var t models.CarpoolTripWithPassengers
		if err := rows.Scan(
			&t.ID, &t.VehicleID, &t.DriveID, &t.TripGroupID, &t.Title, &t.Date, &t.DistanceKm,
			&t.ElectricityCost, &t.TollsCost, &t.TiresCost, &t.MaintenanceCost, &t.InsuranceCost, &t.OtherCost,
			&t.TotalCost, &t.TotalRevenue, &t.NetCost, &t.Notes, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		t.Passengers = []models.CarpoolPassenger{}
		trips = append(trips, t)
	}

	// Fetch passengers for all trips
	for i := range trips {
		pRows, err := r.pool.Query(ctx, `
			SELECT id, carpool_trip_id, passenger_name, origin, destination, seats, amount_paid, notes, created_at
			FROM carpool_passengers
			WHERE carpool_trip_id = $1
			ORDER BY created_at ASC;
		`, trips[i].ID)
		if err == nil {
			for pRows.Next() {
				var p models.CarpoolPassenger
				if err := pRows.Scan(
					&p.ID, &p.CarpoolTripID, &p.PassengerName, &p.Origin, &p.Destination,
					&p.Seats, &p.AmountPaid, &p.Notes, &p.CreatedAt,
				); err == nil {
					trips[i].Passengers = append(trips[i].Passengers, p)
				}
			}
			pRows.Close()
		}
	}

	if trips == nil {
		trips = []models.CarpoolTripWithPassengers{}
	}
	return trips, nil
}

func (r *Repository) GetCarpoolTrip(ctx context.Context, id, vehicleID string) (*models.CarpoolTripWithPassengers, error) {
	query := `
		SELECT id, vehicle_id, drive_id, trip_group_id, title, date, distance_km,
		       electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost,
		       total_cost, total_revenue, net_cost, notes, created_at, updated_at
		FROM carpool_trips
		WHERE id = $1 AND vehicle_id = $2;
	`
	var t models.CarpoolTripWithPassengers
	err := r.pool.QueryRow(ctx, query, id, vehicleID).Scan(
		&t.ID, &t.VehicleID, &t.DriveID, &t.TripGroupID, &t.Title, &t.Date, &t.DistanceKm,
		&t.ElectricityCost, &t.TollsCost, &t.TiresCost, &t.MaintenanceCost, &t.InsuranceCost, &t.OtherCost,
		&t.TotalCost, &t.TotalRevenue, &t.NetCost, &t.Notes, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get carpool trip: %w", err)
	}

	t.Passengers = []models.CarpoolPassenger{}
	pRows, err := r.pool.Query(ctx, `
		SELECT id, carpool_trip_id, passenger_name, origin, destination, seats, amount_paid, notes, created_at
		FROM carpool_passengers
		WHERE carpool_trip_id = $1
		ORDER BY created_at ASC;
	`, t.ID)
	if err == nil {
		defer pRows.Close()
		for pRows.Next() {
			var p models.CarpoolPassenger
			if err := pRows.Scan(
				&p.ID, &p.CarpoolTripID, &p.PassengerName, &p.Origin, &p.Destination,
				&p.Seats, &p.AmountPaid, &p.Notes, &p.CreatedAt,
			); err == nil {
				t.Passengers = append(t.Passengers, p)
			}
		}
	}

	return &t, nil
}

func (r *Repository) UpdateCarpoolTrip(ctx context.Context, trip *models.CarpoolTrip, passengers []models.CarpoolPassenger) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var revenue float64
	for _, p := range passengers {
		revenue += p.AmountPaid
	}
	trip.TotalRevenue = revenue
	trip.TotalCost = trip.ElectricityCost + trip.TollsCost + trip.TiresCost + trip.MaintenanceCost + trip.InsuranceCost + trip.OtherCost
	trip.NetCost = trip.TotalCost - trip.TotalRevenue

	query := `
		UPDATE carpool_trips
		SET drive_id = $1, trip_group_id = $2, title = $3, date = $4, distance_km = $5,
		    electricity_cost = $6, tolls_cost = $7, tires_cost = $8, maintenance_cost = $9,
		    insurance_cost = $10, other_cost = $11, total_cost = $12, total_revenue = $13,
		    net_cost = $14, notes = $15, updated_at = NOW()
		WHERE id = $16 AND vehicle_id = $17
		RETURNING updated_at;
	`
	err = tx.QueryRow(ctx, query,
		trip.DriveID, trip.TripGroupID, trip.Title, trip.Date, trip.DistanceKm,
		trip.ElectricityCost, trip.TollsCost, trip.TiresCost, trip.MaintenanceCost,
		trip.InsuranceCost, trip.OtherCost, trip.TotalCost, trip.TotalRevenue,
		trip.NetCost, trip.Notes, trip.ID, trip.VehicleID,
	).Scan(&trip.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to update carpool trip: %w", err)
	}

	// Delete and recreate passengers
	if _, err := tx.Exec(ctx, `DELETE FROM carpool_passengers WHERE carpool_trip_id = $1;`, trip.ID); err != nil {
		return fmt.Errorf("failed to clear old passengers: %w", err)
	}

	for i := range passengers {
		p := &passengers[i]
		p.CarpoolTripID = trip.ID
		pQuery := `
			INSERT INTO carpool_passengers (
				carpool_trip_id, passenger_name, origin, destination, seats, amount_paid, notes
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, created_at;
		`
		if err := tx.QueryRow(ctx, pQuery,
			p.CarpoolTripID, p.PassengerName, p.Origin, p.Destination, p.Seats, p.AmountPaid, p.Notes,
		).Scan(&p.ID, &p.CreatedAt); err != nil {
			return fmt.Errorf("failed to insert passenger: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) DeleteCarpoolTrip(ctx context.Context, id, vehicleID string) error {
	query := `DELETE FROM carpool_trips WHERE id = $1 AND vehicle_id = $2;`
	cmd, err := r.pool.Exec(ctx, query, id, vehicleID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) GetCarpoolSummary(ctx context.Context, vehicleID string) (*models.CarpoolSummary, error) {
	query := `
		SELECT
			COUNT(*) AS total_trips,
			COALESCE(SUM(distance_km), 0) AS total_distance,
			COALESCE(SUM(total_cost), 0) AS total_cost,
			COALESCE(SUM(total_revenue), 0) AS total_revenue,
			COALESCE(SUM(net_cost), 0) AS total_net_cost
		FROM carpool_trips
		WHERE vehicle_id = $1;
	`
	var s models.CarpoolSummary
	err := r.pool.QueryRow(ctx, query, vehicleID).Scan(
		&s.TotalTrips, &s.TotalDistanceKm, &s.TotalRealCost, &s.TotalRevenue, &s.TotalNetCost,
	)
	if err != nil {
		return nil, err
	}

	// Count passengers
	_ = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM carpool_passengers cp
		JOIN carpool_trips ct ON cp.carpool_trip_id = ct.id
		WHERE ct.vehicle_id = $1;
	`, vehicleID).Scan(&s.TotalPassengers)

	if s.TotalRealCost > 0 {
		s.CoverageRatePct = math.Round((s.TotalRevenue/s.TotalRealCost)*1000) / 10
		s.TotalSaved = s.TotalRevenue
	}
	if s.TotalDistanceKm > 0 {
		s.NetCostPerKm = math.Round((s.TotalNetCost/s.TotalDistanceKm)*1000) / 1000
	}

	return &s, nil
}

func (r *Repository) GetDriveByID(ctx context.Context, driveID, vehicleID string) (*models.Drive, error) {
	query := `
		SELECT id, vehicle_id, teslamate_drive_id, start_time, end_time,
		       start_odometer, end_odometer, distance_km, duration_min,
		       speed_avg, start_address, end_address, energy_consumed_kwh,
		       consumption_kwh_100km, tags, is_manual, created_at, updated_at
		FROM drives
		WHERE id = $1 AND vehicle_id = $2;
	`
	var d models.Drive
	err := r.pool.QueryRow(ctx, query, driveID, vehicleID).Scan(
		&d.ID, &d.VehicleID, &d.TeslaMateDriveID, &d.StartTime, &d.EndTime,
		&d.StartOdometer, &d.EndOdometer, &d.DistanceKm, &d.DurationMin,
		&d.SpeedAvg, &d.StartAddress, &d.EndAddress, &d.EnergyConsumedKwh,
		&d.ConsumptionKwh100km, &d.Tags, &d.IsManual, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (r *Repository) GetTripGroupDrives(ctx context.Context, tripGroupID string) ([]models.Drive, error) {
	query := `
		SELECT d.id, d.vehicle_id, d.teslamate_drive_id, d.start_time, d.end_time,
		       d.start_odometer, d.end_odometer, d.distance_km, d.duration_min,
		       d.speed_avg, d.start_address, d.end_address, d.energy_consumed_kwh,
		       d.consumption_kwh_100km, d.tags, d.is_manual, d.created_at, d.updated_at
		FROM drives d
		JOIN trip_group_drives tgd ON d.id = tgd.drive_id
		WHERE tgd.trip_group_id = $1
		ORDER BY tgd.order_index ASC;
	`
	rows, err := r.pool.Query(ctx, query, tripGroupID)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		list = append(list, d)
	}
	return list, nil
}

func (r *Repository) GetTollExpensesForDriveOrGroup(ctx context.Context, vehicleID string, driveID, tripGroupID *string) (float64, error) {
	var total float64
	if driveID != nil && *driveID != "" {
		_ = r.pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount), 0)
			FROM drive_expenses
			WHERE vehicle_id = $1 AND drive_id = $2;
		`, vehicleID, *driveID).Scan(&total)
	} else if tripGroupID != nil && *tripGroupID != "" {
		_ = r.pool.QueryRow(ctx, `
			SELECT COALESCE(SUM(amount), 0)
			FROM drive_expenses
			WHERE vehicle_id = $1 AND trip_group_id = $2;
		`, vehicleID, *tripGroupID).Scan(&total)
	}
	return total, nil
}

