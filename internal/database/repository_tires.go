package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// Tire lifecycle: mounts, sessions, rotations, disposal and wear logs.

func (r *Repository) CreateTire(ctx context.Context, t *models.Tire) error {
	return r.CreateTiresBatch(ctx, []*models.Tire{t})
}

func (r *Repository) ListTires(ctx context.Context, vehicleID string) ([]models.Tire, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+tireColumns+`
		FROM tires
		WHERE vehicle_id = $1 AND is_archived = FALSE
		ORDER BY current_position ASC, purchase_date DESC;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.Tire
	for rows.Next() {
		var t models.Tire
		if err := scanTire(rows, &t); err != nil {
			return nil, err
		}
		list = append(list, t)
	}
	return list, rows.Err()
}

func (r *Repository) GetTireByID(ctx context.Context, id, vehicleID string) (*models.Tire, error) {
	var t models.Tire
	err := scanTire(r.pool.QueryRow(ctx, `
		SELECT `+tireColumns+`
		FROM tires
		WHERE id::text = $1 AND vehicle_id = $2;
	`, id, vehicleID), &t)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

// UpdateTire updates descriptive tire fields and recomputes its lifetime distance.
// Position changes go through mount sessions / rotations, never through this method.
func (r *Repository) UpdateTire(ctx context.Context, t *models.Tire) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := updateTireTx(ctx, tx, t); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}

	updated, err := r.GetTireByID(ctx, t.ID, *t.VehicleID)
	if err != nil {
		return err
	}
	*t = *updated
	return nil
}

func updateTireTx(ctx context.Context, tx pgx.Tx, t *models.Tire) error {
	lifespan := t.EstimatedLifespanKm
	if lifespan <= 0 {
		lifespan = 40000
	}
	tag, err := tx.Exec(ctx, `
		UPDATE tires
		SET brand = $1, model = $2, dimension = $3, season = $4,
		    purchase_date = $5, purchase_price = $6,
		    initial_depth_mm = $7, min_legal_depth_mm = $8, dot_code = $9,
		    initial_distance_km = $10, estimated_lifespan_km = $11,
		    updated_at = NOW()
		WHERE id::text = $12 AND vehicle_id = $13;
	`,
		t.Brand, t.Model, t.Dimension, t.Season,
		t.PurchaseDate, t.PurchasePrice,
		t.InitialDepthMm, t.MinLegalDepthMm, t.DotCode,
		t.InitialDistanceKm, lifespan,
		t.ID, t.VehicleID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return recalcTireDistance(ctx, tx, t.ID)
}

// TirePatch holds the fields applied to several tires at once; nil fields are left unchanged.
type TirePatch struct {
	Brand               *string
	Model               *string
	Dimension           *string
	Season              *models.TireSeason
	PurchaseDate        *time.Time
	PurchasePrice       *money.Cents // Unit price
	TotalPrice          *money.Cents // Split to the cent across the selected tires (overrides PurchasePrice)
	InitialDepthMm      *float64
	MinLegalDepthMm     *float64
	DotCode             *string
	InitialDistanceKm   *float64
	EstimatedLifespanKm *int
	// Active mount session of mounted tires
	MountedDate     *time.Time
	MountedOdometer *float64
}

// BatchUpdateTires applies a patch to several tires of a vehicle in one transaction.
func (r *Repository) BatchUpdateTires(ctx context.Context, vehicleID string, tireIDs []string, p TirePatch) error {
	ids := uniqueStrings(tireIDs)
	if len(ids) == 0 {
		return validationErrorf("aucun pneu sélectionné")
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	current, err := lockVehicleTires(ctx, tx, vehicleID)
	if err != nil {
		return err
	}
	var prices []money.Cents
	if p.TotalPrice != nil {
		prices = money.Split(*p.TotalPrice, len(ids))
	}

	for i, id := range ids {
		t, ok := current[id]
		if !ok {
			return ErrForeignReference
		}
		if p.Brand != nil {
			t.Brand = *p.Brand
		}
		if p.Model != nil {
			t.Model = *p.Model
		}
		if p.Dimension != nil {
			t.Dimension = *p.Dimension
		}
		if p.Season != nil {
			t.Season = *p.Season
		}
		if p.PurchaseDate != nil {
			t.PurchaseDate = *p.PurchaseDate
		}
		if prices != nil {
			t.PurchasePrice = prices[i]
		} else if p.PurchasePrice != nil {
			t.PurchasePrice = *p.PurchasePrice
		}
		if p.InitialDepthMm != nil {
			t.InitialDepthMm = *p.InitialDepthMm
		}
		if p.MinLegalDepthMm != nil {
			t.MinLegalDepthMm = *p.MinLegalDepthMm
		}
		if p.DotCode != nil {
			t.DotCode = p.DotCode
		}
		if p.InitialDistanceKm != nil {
			t.InitialDistanceKm = *p.InitialDistanceKm
		}
		if p.EstimatedLifespanKm != nil {
			t.EstimatedLifespanKm = *p.EstimatedLifespanKm
		}
		t.VehicleID = &vehicleID
		if err := updateTireTx(ctx, tx, t); err != nil {
			return err
		}

		if (p.MountedDate != nil || p.MountedOdometer != nil) && isMountedPosition(t.CurrentPosition) {
			if err := setActiveMount(ctx, tx, t, vehicleID, p.MountedDate, p.MountedOdometer); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

// setActiveMount updates the open mount session of a mounted tire, creating it when missing.
func setActiveMount(ctx context.Context, tx pgx.Tx, t *models.Tire, vehicleID string, date *time.Time, odometer *float64) error {
	var sessionID string
	var curDate time.Time
	var curOdo float64
	err := tx.QueryRow(ctx, `
		SELECT id, mounted_date, mounted_odometer FROM tire_mount_sessions
		WHERE tire_id::text = $1 AND dismounted_date IS NULL
		ORDER BY mounted_date DESC LIMIT 1;
	`, t.ID).Scan(&sessionID, &curDate, &curOdo)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if date == nil || odometer == nil {
			return validationErrorf("le pneu %s %s n'a pas de montage en cours : date et odomètre de montage requis", t.Brand, t.CurrentPosition)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO tire_mount_sessions (tire_id, vehicle_id, position, mounted_date, mounted_odometer)
			VALUES ($1, $2, $3, $4, $5);
		`, t.ID, vehicleID, t.CurrentPosition, *date, *odometer); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if date != nil {
			curDate = *date
		}
		if odometer != nil {
			curOdo = *odometer
		}
		if _, err := tx.Exec(ctx, `
			UPDATE tire_mount_sessions SET mounted_date = $1, mounted_odometer = $2, updated_at = NOW() WHERE id = $3;
		`, curDate, curOdo, sessionID); err != nil {
			return err
		}
	}
	return recalcTireDistance(ctx, tx, t.ID)
}

// DeleteTire permanently deletes a tire with its sessions and wear logs (erroneous entry).
func (r *Repository) DeleteTire(ctx context.Context, vehicleID, tireID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM tires WHERE id::text = $1 AND vehicle_id = $2;`, tireID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DisposeTire retires a tire (worn out, damaged, sold): closes its mount session and moves it to DISPOSED.
// Its purchase price is then fully counted as consumed in the amortized cost.
func (r *Repository) DisposeTire(ctx context.Context, vehicleID, tireID string, at time.Time, odometer *float64) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	current, err := lockVehicleTires(ctx, tx, vehicleID)
	if err != nil {
		return err
	}
	t, ok := current[tireID]
	if !ok {
		return ErrNotFound
	}
	if isMountedPosition(t.CurrentPosition) {
		if odometer == nil {
			return validationErrorf("l'odomètre de démontage est requis pour un pneu monté")
		}
		if t.MountedOdometer != nil && *odometer < *t.MountedOdometer {
			return validationErrorf("l'odomètre (%.0f km) est inférieur à l'odomètre de montage (%.0f km)", *odometer, *t.MountedOdometer)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE tire_mount_sessions
			SET dismounted_date = $1, dismounted_odometer = $2,
			    distance_km = GREATEST($2 - mounted_odometer, 0), updated_at = NOW()
			WHERE tire_id = $3 AND dismounted_date IS NULL;
		`, at, *odometer, tireID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE tires SET current_position = 'DISPOSED', updated_at = NOW() WHERE id = $1;`, tireID); err != nil {
		return err
	}
	if err := recalcTireDistance(ctx, tx, tireID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// BatchDisposeTires retires multiple tires in a single transaction.
func (r *Repository) BatchDisposeTires(ctx context.Context, vehicleID string, tireIDs []string, at time.Time, odometer *float64) error {
	if len(tireIDs) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	current, err := lockVehicleTires(ctx, tx, vehicleID)
	if err != nil {
		return err
	}

	for _, tireID := range tireIDs {
		t, ok := current[tireID]
		if !ok {
			return ErrNotFound
		}
		if isMountedPosition(t.CurrentPosition) {
			if odometer == nil {
				return validationErrorf("l'odomètre de démontage est requis pour le pneu monté %s", t.Brand)
			}
			if t.MountedOdometer != nil && *odometer < *t.MountedOdometer {
				return validationErrorf("l'odomètre (%.0f km) est inférieur à l'odomètre de montage (%.0f km) pour %s", *odometer, *t.MountedOdometer, t.Brand)
			}
			if _, err := tx.Exec(ctx, `
				UPDATE tire_mount_sessions
				SET dismounted_date = $1, dismounted_odometer = $2,
				    distance_km = GREATEST($2 - mounted_odometer, 0), updated_at = NOW()
				WHERE tire_id = $3 AND dismounted_date IS NULL;
			`, at, *odometer, tireID); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE tires SET current_position = 'DISPOSED', updated_at = NOW() WHERE id = $1;`, tireID); err != nil {
			return err
		}
		if err := recalcTireDistance(ctx, tx, tireID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

// CopyTireHistory duplicates all sessions and/or tread logs from a source tire to one or more target tires.
func (r *Repository) CopyTireHistory(ctx context.Context, vehicleID, sourceTireID string, targetTireIDs []string, copySessions, copyLogs, adaptPosition bool) error {
	if len(targetTireIDs) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	current, err := lockVehicleTires(ctx, tx, vehicleID)
	if err != nil {
		return err
	}
	source, ok := current[sourceTireID]
	if !ok {
		return ErrNotFound
	}
	_ = source

	// Fetch sessions of source tire
	type sessionRow struct {
		position           models.TirePosition
		mountedDate        time.Time
		mountedOdometer    float64
		dismountedDate     *time.Time
		dismountedOdometer *float64
		distanceKm         float64
		notes              *string
	}
	var sourceSessions []sessionRow
	if copySessions {
		rows, err := tx.Query(ctx, `
			SELECT position, mounted_date, mounted_odometer, dismounted_date, dismounted_odometer, distance_km, notes
			FROM tire_mount_sessions
			WHERE tire_id = $1 AND vehicle_id = $2
			ORDER BY mounted_date ASC;
		`, sourceTireID, vehicleID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var s sessionRow
			if err := rows.Scan(&s.position, &s.mountedDate, &s.mountedOdometer, &s.dismountedDate, &s.dismountedOdometer, &s.distanceKm, &s.notes); err != nil {
				rows.Close()
				return err
			}
			sourceSessions = append(sourceSessions, s)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}

	// Fetch logs of source tire
	type logRow struct {
		date     time.Time
		odometer float64
		depthMm  float64
		notes    *string
	}
	var sourceLogs []logRow
	if copyLogs {
		rows, err := tx.Query(ctx, `
			SELECT date, odometer, depth_mm, notes
			FROM tire_logs
			WHERE tire_id = $1
			ORDER BY date ASC;
		`, sourceTireID)
		if err != nil {
			return err
		}
		for rows.Next() {
			var l logRow
			if err := rows.Scan(&l.date, &l.odometer, &l.depthMm, &l.notes); err != nil {
				rows.Close()
				return err
			}
			sourceLogs = append(sourceLogs, l)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}

	for _, targetID := range targetTireIDs {
		if targetID == sourceTireID {
			continue
		}
		target, ok := current[targetID]
		if !ok {
			return ErrNotFound
		}

		if copySessions {
			for _, s := range sourceSessions {
				pos := s.position
				if adaptPosition && isMountedPosition(target.CurrentPosition) {
					pos = target.CurrentPosition
				}
				if _, err := tx.Exec(ctx, `
					INSERT INTO tire_mount_sessions (
						tire_id, vehicle_id, position, mounted_date, mounted_odometer,
						dismounted_date, dismounted_odometer, distance_km, notes, created_at, updated_at
					) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW());
				`, targetID, vehicleID, pos, s.mountedDate, s.mountedOdometer, s.dismountedDate, s.dismountedOdometer, s.distanceKm, s.notes); err != nil {
					return err
				}
			}
		}

		if copyLogs {
			for _, l := range sourceLogs {
				if _, err := tx.Exec(ctx, `
					INSERT INTO tire_logs (tire_id, date, odometer, depth_mm, notes, created_at)
					VALUES ($1, $2, $3, $4, $5, NOW());
				`, targetID, l.date, l.odometer, l.depthMm, l.notes); err != nil {
					return err
				}
			}
		}

		if err := recalcTireDistance(ctx, tx, targetID); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

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

// CreateTiresBatch inserts tires and their initial mount sessions atomically.
// AccumulatedDistanceKm provided at creation is recorded as the tire's initial distance.
func (r *Repository) CreateTiresBatch(ctx context.Context, tires []*models.Tire) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Two tires must never end up on the same wheel: lock the vehicle's existing
	// tires so a concurrent insert can't slip a second one onto the same position.
	occupied := make(map[models.TirePosition]string)
	if len(tires) > 0 && tires[0].VehicleID != nil {
		existing, err := lockVehicleTires(ctx, tx, *tires[0].VehicleID)
		if err != nil {
			return err
		}
		for _, et := range existing {
			if isMountedPosition(et.CurrentPosition) {
				occupied[et.CurrentPosition] = et.Brand + " " + et.Model
			}
		}
	}

	for _, t := range tires {
		lifespan := t.EstimatedLifespanKm
		if lifespan <= 0 {
			lifespan = 40000
		}
		if !isValidTirePosition(t.CurrentPosition) {
			return validationErrorf("position de pneu invalide : %s", t.CurrentPosition)
		}
		if isMountedPosition(t.CurrentPosition) && (t.MountedOdometer == nil || t.VehicleID == nil) {
			return validationErrorf("un pneu monté requiert l'odomètre de montage")
		}
		if isMountedPosition(t.CurrentPosition) {
			if other, taken := occupied[t.CurrentPosition]; taken {
				return validationErrorf("la position %s est déjà occupée par %s : mettez-le au rebut ou changez sa position avant d'en monter un nouveau", t.CurrentPosition, other)
			}
			occupied[t.CurrentPosition] = t.Brand + " " + t.Model
		}
		if !isMountedPosition(t.CurrentPosition) {
			t.MountedOdometer = nil
		}
		t.InitialDistanceKm = math.Max(0, t.AccumulatedDistanceKm)

		query := `
			INSERT INTO tires (
				vehicle_id, brand, model, dimension, season,
				purchase_date, purchase_price, current_position,
				initial_depth_mm, min_legal_depth_mm, dot_code,
				mounted_odometer, initial_distance_km, accumulated_distance_km, estimated_lifespan_km
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $13, $14)
			RETURNING id, created_at, updated_at;
		`
		if err := tx.QueryRow(ctx, query,
			t.VehicleID, t.Brand, t.Model, t.Dimension, t.Season,
			t.PurchaseDate, t.PurchasePrice, t.CurrentPosition,
			t.InitialDepthMm, t.MinLegalDepthMm, t.DotCode,
			t.MountedOdometer, t.InitialDistanceKm, lifespan,
		).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return fmt.Errorf("failed to insert tire: %w", err)
		}
		t.AccumulatedDistanceKm = t.InitialDistanceKm
		t.EstimatedLifespanKm = lifespan

		if isMountedPosition(t.CurrentPosition) {
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
		WHERE tire_id::text = $1
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
	return list, rows.Err()
}

func normalizeSessionDistance(s *models.TireMountSession) error {
	if s.DismountedOdometer != nil {
		if *s.DismountedOdometer < s.MountedOdometer {
			return validationErrorf("l'odomètre de démontage est inférieur à celui du montage")
		}
		// Odometers win over a manual distance; a manual distance is kept when odometers are unknown.
		if *s.DismountedOdometer > s.MountedOdometer {
			s.DistanceKm = *s.DismountedOdometer - s.MountedOdometer
		}
	} else if s.DismountedDate == nil {
		s.DistanceKm = 0
	}
	if s.DistanceKm < 0 {
		return validationErrorf("la distance d'une session ne peut pas être négative")
	}
	return nil
}

func (r *Repository) CreateTireMountSession(ctx context.Context, s *models.TireMountSession) error {
	if err := normalizeSessionDistance(s); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ensureTiresOwned(ctx, tx, s.VehicleID, []string{s.TireID}); err != nil {
		return ErrNotFound
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO tire_mount_sessions (
			tire_id, vehicle_id, position, mounted_date, mounted_odometer,
			dismounted_date, dismounted_odometer, distance_km, notes
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at;
	`,
		s.TireID, s.VehicleID, s.Position, s.MountedDate, s.MountedOdometer,
		s.DismountedDate, s.DismountedOdometer, s.DistanceKm, s.Notes,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return err
	}
	if err := recalcTireDistance(ctx, tx, s.TireID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) UpdateTireMountSession(ctx context.Context, s *models.TireMountSession) error {
	if err := normalizeSessionDistance(s); err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE tire_mount_sessions s
		SET position = $1, mounted_date = $2, mounted_odometer = $3,
		    dismounted_date = $4, dismounted_odometer = $5, distance_km = $6, notes = $7,
		    updated_at = NOW()
		FROM tires t
		WHERE s.id::text = $8 AND s.tire_id = t.id AND t.id::text = $9 AND t.vehicle_id = $10;
	`,
		s.Position, s.MountedDate, s.MountedOdometer,
		s.DismountedDate, s.DismountedOdometer, s.DistanceKm, s.Notes,
		s.ID, s.TireID, s.VehicleID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := recalcTireDistance(ctx, tx, s.TireID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) DeleteTireMountSession(ctx context.Context, vehicleID, sessionID, tireID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		DELETE FROM tire_mount_sessions s
		USING tires t
		WHERE s.id::text = $1 AND s.tire_id = t.id AND t.id::text = $2 AND t.vehicle_id = $3;
	`, sessionID, tireID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := recalcTireDistance(ctx, tx, tireID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// recalcTireDistance derives accumulated distance (initial + closed sessions) and the active mount odometer.
func recalcTireDistance(ctx context.Context, tx pgx.Tx, tireID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE tires t
		SET accumulated_distance_km = t.initial_distance_km + COALESCE((
		        SELECT SUM(s.distance_km) FROM tire_mount_sessions s
		        WHERE s.tire_id = t.id AND s.dismounted_date IS NOT NULL
		    ), 0),
		    mounted_odometer = (
		        SELECT s.mounted_odometer FROM tire_mount_sessions s
		        WHERE s.tire_id = t.id AND s.dismounted_date IS NULL
		        ORDER BY s.mounted_date DESC LIMIT 1
		    ),
		    updated_at = NOW()
		WHERE t.id::text = $1;
	`, tireID)
	if err != nil {
		return fmt.Errorf("failed to recalculate tire distance: %w", err)
	}
	return nil
}

// QuickRotateTires applies a predefined rotation or a seasonal pack swap.
func (r *Repository) QuickRotateTires(ctx context.Context, vehicleID string, mode string, odometer float64, swapWithPackTireIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	current, err := lockVehicleTires(ctx, tx, vehicleID)
	if err != nil {
		return err
	}

	mountedMap := make(map[models.TirePosition]*models.Tire)
	for _, t := range current {
		if isMountedPosition(t.CurrentPosition) {
			mountedMap[t.CurrentPosition] = t
		}
	}

	newPositions := make(map[string]models.TirePosition)
	swap := func(a, b models.TirePosition) {
		if t, ok := mountedMap[a]; ok {
			newPositions[t.ID] = b
		}
		if t, ok := mountedMap[b]; ok {
			newPositions[t.ID] = a
		}
	}

	switch mode {
	case "FRONT_BACK":
		swap(models.TirePosFL, models.TirePosRL)
		swap(models.TirePosFR, models.TirePosRR)
	case "CROSS":
		swap(models.TirePosFL, models.TirePosRR)
		swap(models.TirePosFR, models.TirePosRL)
	case "SWAP_PACK":
		ids := uniqueStrings(swapWithPackTireIDs)
		if len(ids) == 0 || len(ids) > 4 {
			return validationErrorf("un échange de train requiert entre 1 et 4 pneus")
		}
		for _, t := range mountedMap {
			newPositions[t.ID] = models.TirePosStorage
		}
		positions := []models.TirePosition{models.TirePosFL, models.TirePosFR, models.TirePosRL, models.TirePosRR}
		for i, tireID := range ids {
			t, ok := current[tireID]
			if !ok {
				return ErrForeignReference
			}
			if isMountedPosition(t.CurrentPosition) || t.CurrentPosition == models.TirePosDisposed {
				return validationErrorf("le pneu %s n'est pas en stockage", tireID)
			}
			newPositions[tireID] = positions[i]
		}
	default:
		return validationErrorf("mode de permutation inconnu : %s", mode)
	}

	notes := fmt.Sprintf("Permutation rapide: %s", mode)
	return r.applyTirePositions(ctx, tx, vehicleID, current, newPositions, time.Now().UTC(), odometer, &notes)
}

// AddTireRotation applies an explicit position mapping ({"FL": "<tire id>", ...}).
func (r *Repository) AddTireRotation(ctx context.Context, rot *models.TireRotation) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	current, err := lockVehicleTires(ctx, tx, rot.VehicleID)
	if err != nil {
		return err
	}

	newPositions := make(map[string]models.TirePosition)
	for posStr, tireIDVal := range rot.MappingJSON {
		pos := models.TirePosition(posStr)
		tireID, ok := tireIDVal.(string)
		if !ok || tireID == "" {
			continue
		}
		if !isValidTirePosition(pos) {
			return validationErrorf("position de pneu invalide : %s", posStr)
		}
		if _, owned := current[tireID]; !owned {
			return ErrForeignReference
		}
		newPositions[tireID] = pos
	}

	return r.applyTirePositions(ctx, tx, rot.VehicleID, current, newPositions, rot.Date, rot.Odometer, rot.Notes)
}

func lockVehicleTires(ctx context.Context, tx pgx.Tx, vehicleID string) (map[string]*models.Tire, error) {
	rows, err := tx.Query(ctx, `
		SELECT `+tireColumns+`
		FROM tires
		WHERE vehicle_id = $1 AND is_archived = FALSE
		FOR UPDATE;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tires := make(map[string]*models.Tire)
	for rows.Next() {
		var t models.Tire
		if err := scanTire(rows, &t); err != nil {
			return nil, err
		}
		tires[t.ID] = &t
	}
	return tires, rows.Err()
}

// applyTirePositions closes/opens mount sessions for every moved tire, updates positions,
// recomputes distances and records the rotation, all inside the caller's transaction.
func (r *Repository) applyTirePositions(ctx context.Context, tx pgx.Tx, vehicleID string, current map[string]*models.Tire, newPositions map[string]models.TirePosition, at time.Time, odometer float64, notes *string) error {
	if len(newPositions) == 0 {
		return validationErrorf("aucun pneu à déplacer")
	}
	if odometer <= 0 {
		return validationErrorf("un relevé d'odomètre valide est requis")
	}
	for tireID := range newPositions {
		if t := current[tireID]; t.MountedOdometer != nil && odometer < *t.MountedOdometer {
			return validationErrorf("l'odomètre (%.0f km) est inférieur à l'odomètre de montage du pneu %s (%.0f km)", odometer, tireID, *t.MountedOdometer)
		}
	}

	// Two tires must never end up on the same wheel.
	finalPositions := make(map[models.TirePosition]string)
	for id, t := range current {
		pos := t.CurrentPosition
		if p, moved := newPositions[id]; moved {
			pos = p
		}
		if isMountedPosition(pos) {
			if other, taken := finalPositions[pos]; taken {
				return validationErrorf("la position %s serait occupée par deux pneus (%s et %s)", pos, other, id)
			}
			finalPositions[pos] = id
		}
	}

	mapping := make(map[string]any)
	var stored []string
	for tireID, newPos := range newPositions {
		t := current[tireID]
		if newPos == t.CurrentPosition {
			continue
		}
		if isMountedPosition(newPos) {
			mapping[string(newPos)] = tireID
		} else {
			stored = append(stored, tireID)
		}

		if isMountedPosition(t.CurrentPosition) {
			if _, err := tx.Exec(ctx, `
				UPDATE tire_mount_sessions
				SET dismounted_date = $1, dismounted_odometer = $2,
				    distance_km = GREATEST($2 - mounted_odometer, 0), updated_at = NOW()
				WHERE tire_id = $3 AND dismounted_date IS NULL;
			`, at, odometer, tireID); err != nil {
				return fmt.Errorf("failed to close mount session of tire %s: %w", tireID, err)
			}
		}
		if isMountedPosition(newPos) {
			if _, err := tx.Exec(ctx, `
				INSERT INTO tire_mount_sessions (tire_id, vehicle_id, position, mounted_date, mounted_odometer)
				VALUES ($1, $2, $3, $4, $5);
			`, tireID, vehicleID, newPos, at, odometer); err != nil {
				return fmt.Errorf("failed to open mount session of tire %s: %w", tireID, err)
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE tires SET current_position = $1, updated_at = NOW() WHERE id = $2;`, newPos, tireID); err != nil {
			return fmt.Errorf("failed to update position of tire %s: %w", tireID, err)
		}
		if err := recalcTireDistance(ctx, tx, tireID); err != nil {
			return err
		}
	}
	if len(stored) > 0 {
		mapping[string(models.TirePosStorage)] = stored
	}

	mapBytes, err := json.Marshal(mapping)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO tire_rotations (vehicle_id, date, odometer, mapping_json, notes)
		VALUES ($1, $2, $3, $4, $5);
	`, vehicleID, at, odometer, mapBytes, notes); err != nil {
		return fmt.Errorf("failed to record tire rotation: %w", err)
	}

	return tx.Commit(ctx)
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
