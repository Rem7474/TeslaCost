package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
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
		RETURNING id, email, password_hash, oidc_subject, oidc_provider, display_name, created_at, updated_at;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, email, passwordHash).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.OIDCSubject, &u.OIDCProvider, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	return &u, nil
}

func (r *Repository) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, oidc_subject, oidc_provider, display_name, created_at, updated_at
		FROM users
		WHERE email = $1;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.OIDCSubject, &u.OIDCProvider, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt,
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
		SELECT id, email, password_hash, oidc_subject, oidc_provider, display_name, created_at, updated_at
		FROM users
		WHERE id = $1;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.OIDCSubject, &u.OIDCProvider, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt,
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

// GetUserByOIDCSubject looks up a user by their IdP-issued subject claim.
// This is the primary OIDC identity lookup — email alone is not sufficient
// as the same email can appear across different providers.
func (r *Repository) GetUserByOIDCSubject(ctx context.Context, provider, subject string) (*models.User, error) {
	query := `
		SELECT id, email, password_hash, oidc_subject, oidc_provider, display_name, created_at, updated_at
		FROM users
		WHERE oidc_provider = $1 AND oidc_subject = $2;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, provider, subject).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.OIDCSubject, &u.OIDCProvider, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by OIDC subject: %w", err)
	}
	return &u, nil
}

// UpsertOIDCUser performs JIT (Just-In-Time) provisioning:
//   - If a user with the same (oidc_provider, oidc_subject) already exists → update email/display_name.
//   - If a local user with the same email exists → link it to this OIDC identity.
//   - Otherwise → create a new user account with no local password.
func (r *Repository) UpsertOIDCUser(ctx context.Context, email, subject, provider, displayName string) (*models.User, error) {
	query := `
		INSERT INTO users (email, oidc_subject, oidc_provider, display_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (oidc_provider, oidc_subject) WHERE oidc_subject IS NOT NULL
		DO UPDATE SET
			email        = EXCLUDED.email,
			display_name = EXCLUDED.display_name,
			updated_at   = NOW()
		RETURNING id, email, password_hash, oidc_subject, oidc_provider, display_name, created_at, updated_at;
	`
	var u models.User
	err := r.pool.QueryRow(ctx, query, email, subject, provider, displayName).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.OIDCSubject, &u.OIDCProvider, &u.DisplayName, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		// Conflict on email (local account with same email exists but no OIDC link yet).
		// Link the existing account to this OIDC identity.
		linkQuery := `
			UPDATE users
			SET oidc_subject  = $1,
			    oidc_provider = $2,
			    display_name  = COALESCE($3, display_name),
			    updated_at    = NOW()
			WHERE email = $4
			RETURNING id, email, password_hash, oidc_subject, oidc_provider, display_name, created_at, updated_at;
		`
		var linked models.User
		linkErr := r.pool.QueryRow(ctx, linkQuery, subject, provider, displayName, email).Scan(
			&linked.ID, &linked.Email, &linked.PasswordHash, &linked.OIDCSubject, &linked.OIDCProvider, &linked.DisplayName, &linked.CreatedAt, &linked.UpdatedAt,
		)
		if linkErr != nil {
			return nil, fmt.Errorf("failed to upsert OIDC user: insert=%w, link=%v", err, linkErr)
		}
		return &linked, nil
	}
	return &u, nil
}

// ============================================================================
// Refresh Tokens
// ============================================================================

// CreateRefreshToken inserts a new active refresh token record.
func (r *Repository) CreateRefreshToken(ctx context.Context, userID, tokenHash, familyID string, expiresAt time.Time, ip, userAgent *string) (*models.RefreshToken, error) {
	query := `
		INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at, created_ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, token_hash, family_id, is_revoked, expires_at, created_at, created_ip, user_agent;
	`
	var rt models.RefreshToken
	err := r.pool.QueryRow(ctx, query, userID, tokenHash, familyID, expiresAt, ip, userAgent).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash, &rt.FamilyID, &rt.IsRevoked, &rt.ExpiresAt, &rt.CreatedAt, &rt.CreatedIP, &rt.UserAgent,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create refresh token: %w", err)
	}
	return &rt, nil
}

// GetRefreshTokenByHash retrieves a refresh token record by its SHA-256 hash.
func (r *Repository) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	query := `
		SELECT id, user_id, token_hash, family_id, is_revoked, expires_at, created_at, created_ip, user_agent
		FROM refresh_tokens
		WHERE token_hash = $1;
	`
	var rt models.RefreshToken
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&rt.ID, &rt.UserID, &rt.TokenHash, &rt.FamilyID, &rt.IsRevoked, &rt.ExpiresAt, &rt.CreatedAt, &rt.CreatedIP, &rt.UserAgent,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get refresh token: %w", err)
	}
	return &rt, nil
}

// ErrRefreshTokenReused is returned when a previously revoked refresh token is presented again (theft/replay).
var ErrRefreshTokenReused = errors.New("refresh token reuse detected: token was already revoked")

// RotateRefreshToken atomically revokes the old token and issues a new one under the same family.
// If the old token was already revoked, it revokes the entire family and returns ErrRefreshTokenReused.
func (r *Repository) RotateRefreshToken(ctx context.Context, oldTokenHash, newTokenHash string, expiresAt time.Time, ip, userAgent *string) (*models.RefreshToken, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var oldToken models.RefreshToken
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, token_hash, family_id, is_revoked, expires_at, created_at, created_ip, user_agent
		FROM refresh_tokens
		WHERE token_hash = $1
		FOR UPDATE;
	`, oldTokenHash).Scan(
		&oldToken.ID, &oldToken.UserID, &oldToken.TokenHash, &oldToken.FamilyID, &oldToken.IsRevoked, &oldToken.ExpiresAt, &oldToken.CreatedAt, &oldToken.CreatedIP, &oldToken.UserAgent,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to find old refresh token: %w", err)
	}

	// Replay / Reuse Detection: If the old token was already revoked, someone is reusing an old token!
	if oldToken.IsRevoked {
		_, _ = tx.Exec(ctx, `UPDATE refresh_tokens SET is_revoked = TRUE WHERE family_id = $1`, oldToken.FamilyID)
		_ = tx.Commit(ctx)
		return nil, ErrRefreshTokenReused
	}

	// Check if expired
	if time.Now().After(oldToken.ExpiresAt) {
		return nil, errors.New("refresh token expired")
	}

	// Mark old token as revoked
	_, err = tx.Exec(ctx, `UPDATE refresh_tokens SET is_revoked = TRUE WHERE id = $1`, oldToken.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to revoke old refresh token: %w", err)
	}

	// Insert new token with the exact same family_id
	var newToken models.RefreshToken
	err = tx.QueryRow(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at, created_ip, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, token_hash, family_id, is_revoked, expires_at, created_at, created_ip, user_agent;
	`, oldToken.UserID, newTokenHash, oldToken.FamilyID, expiresAt, ip, userAgent).Scan(
		&newToken.ID, &newToken.UserID, &newToken.TokenHash, &newToken.FamilyID, &newToken.IsRevoked, &newToken.ExpiresAt, &newToken.CreatedAt, &newToken.CreatedIP, &newToken.UserAgent,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rotated refresh token: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit token rotation: %w", err)
	}

	return &newToken, nil
}

// RevokeRefreshTokenFamily marks all tokens in a family as revoked.
func (r *Repository) RevokeRefreshTokenFamily(ctx context.Context, familyID string) error {
	query := `UPDATE refresh_tokens SET is_revoked = TRUE WHERE family_id = $1;`
	_, err := r.pool.Exec(ctx, query, familyID)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token family: %w", err)
	}
	return nil
}

// RevokeRefreshToken marks a single refresh token as revoked.
func (r *Repository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	query := `UPDATE refresh_tokens SET is_revoked = TRUE WHERE token_hash = $1;`
	_, err := r.pool.Exec(ctx, query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to revoke refresh token: %w", err)
	}
	return nil
}

// RevokeAllUserRefreshTokens marks all refresh tokens for a user as revoked.
func (r *Repository) RevokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	query := `UPDATE refresh_tokens SET is_revoked = TRUE WHERE user_id = $1;`
	_, err := r.pool.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to revoke all user refresh tokens: %w", err)
	}
	return nil
}

// CleanupExpiredRefreshTokens deletes old expired/revoked refresh tokens older than 7 days.
func (r *Repository) CleanupExpiredRefreshTokens(ctx context.Context) (int64, error) {
	query := `DELETE FROM refresh_tokens WHERE expires_at < NOW() - INTERVAL '7 days' OR (is_revoked = TRUE AND created_at < NOW() - INTERVAL '7 days');`
	tag, err := r.pool.Exec(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to clean up expired refresh tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ============================================================================
// Vehicles
// ============================================================================

const vehicleColumns = `
	id, user_id, name, vin, teslamate_car_id, current_odometer,
	teslamate_api_url, teslamate_auth_type, teslamate_api_key_encrypted,
	teslamate_basic_user, teslamate_basic_pass_encrypted,
	pre_teslamate_kwh_100km, pre_teslamate_eur_per_kwh,
	created_at, updated_at
`

func scanVehicle(row pgx.Row, v *models.Vehicle) error {
	return row.Scan(
		&v.ID, &v.UserID, &v.Name, &v.Vin, &v.TeslaMateCarID, &v.CurrentOdometer,
		&v.TeslaMateAPIURL, &v.TeslaMateAuthType, &v.TeslaMateAPIKeyEncrypted,
		&v.TeslaMateBasicUser, &v.TeslaMateBasicPassEnc,
		&v.PreTeslaMateKwh100km, &v.PreTeslaMateEurPerKwh,
		&v.CreatedAt, &v.UpdatedAt,
	)
}

func sanitizeVehicleForRole(v *models.Vehicle) {
	if v.Role != models.RoleOwner {
		v.TeslaMateAPIURL = nil
		v.TeslaMateAPIKeyEncrypted = nil
		v.TeslaMateBasicUser = nil
		v.TeslaMateBasicPassEnc = nil
	}
}

func (r *Repository) CreateVehicle(ctx context.Context, v *models.Vehicle) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO vehicles (
			user_id, name, vin, teslamate_car_id, current_odometer,
			teslamate_api_url, teslamate_auth_type, teslamate_api_key_encrypted,
			teslamate_basic_user, teslamate_basic_pass_encrypted,
			pre_teslamate_kwh_100km, pre_teslamate_eur_per_kwh
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at;
	`
	err = tx.QueryRow(ctx, query,
		v.UserID, v.Name, v.Vin, v.TeslaMateCarID, v.CurrentOdometer,
		v.TeslaMateAPIURL, v.TeslaMateAuthType, v.TeslaMateAPIKeyEncrypted,
		v.TeslaMateBasicUser, v.TeslaMateBasicPassEnc,
		v.PreTeslaMateKwh100km, v.PreTeslaMateEurPerKwh,
	).Scan(&v.ID, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to create vehicle: %w", err)
	}

	memberQuery := `
		INSERT INTO vehicle_members (vehicle_id, user_id, role)
		VALUES ($1, $2, 'OWNER')
		ON CONFLICT (vehicle_id, user_id) DO NOTHING;
	`
	if _, err := tx.Exec(ctx, memberQuery, v.ID, v.UserID); err != nil {
		return fmt.Errorf("failed to insert vehicle owner member: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit vehicle creation: %w", err)
	}
	v.Role = models.RoleOwner
	return nil
}

func (r *Repository) listVehicles(ctx context.Context, where string, args ...any) ([]models.Vehicle, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+vehicleColumns+` FROM vehicles WHERE `+where+` ORDER BY created_at ASC;`, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list vehicles: %w", err)
	}
	defer rows.Close()

	var list []models.Vehicle
	for rows.Next() {
		var v models.Vehicle
		if err := scanVehicle(rows, &v); err != nil {
			return nil, err
		}
		v.Role = models.RoleOwner
		list = append(list, v)
	}
	return list, rows.Err()
}

func (r *Repository) ListVehiclesByUserID(ctx context.Context, userID string) ([]models.Vehicle, error) {
	query := `
		SELECT v.id, v.user_id, v.name, v.vin, v.teslamate_car_id, v.current_odometer,
		       v.teslamate_api_url, v.teslamate_auth_type, v.teslamate_api_key_encrypted,
		       v.teslamate_basic_user, v.teslamate_basic_pass_encrypted,
		       v.pre_teslamate_kwh_100km, v.pre_teslamate_eur_per_kwh,
		       v.created_at, v.updated_at,
		       COALESCE(vm.role, CASE WHEN v.user_id::text = $1 THEN 'OWNER' ELSE 'VIEWER' END) as role
		FROM vehicles v
		LEFT JOIN vehicle_members vm ON v.id = vm.vehicle_id AND vm.user_id::text = $1
		WHERE v.user_id::text = $1 OR vm.user_id IS NOT NULL
		ORDER BY v.created_at ASC;
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list vehicles: %w", err)
	}
	defer rows.Close()

	var list []models.Vehicle
	for rows.Next() {
		var v models.Vehicle
		var role string
		if err := rows.Scan(
			&v.ID, &v.UserID, &v.Name, &v.Vin, &v.TeslaMateCarID, &v.CurrentOdometer,
			&v.TeslaMateAPIURL, &v.TeslaMateAuthType, &v.TeslaMateAPIKeyEncrypted,
			&v.TeslaMateBasicUser, &v.TeslaMateBasicPassEnc,
			&v.PreTeslaMateKwh100km, &v.PreTeslaMateEurPerKwh,
			&v.CreatedAt, &v.UpdatedAt, &role,
		); err != nil {
			return nil, err
		}
		v.Role = models.VehicleRole(role)
		sanitizeVehicleForRole(&v)
		list = append(list, v)
	}
	return list, rows.Err()
}

func (r *Repository) ListAllVehiclesWithTeslaMate(ctx context.Context) ([]models.Vehicle, error) {
	return r.listVehicles(ctx, `teslamate_api_url IS NOT NULL AND teslamate_api_url != ''`)
}

func (r *Repository) GetVehicleByID(ctx context.Context, id, userID string) (*models.Vehicle, error) {
	var v models.Vehicle
	query := `
		SELECT v.id, v.user_id, v.name, v.vin, v.teslamate_car_id, v.current_odometer,
		       v.teslamate_api_url, v.teslamate_auth_type, v.teslamate_api_key_encrypted,
		       v.teslamate_basic_user, v.teslamate_basic_pass_encrypted,
		       v.pre_teslamate_kwh_100km, v.pre_teslamate_eur_per_kwh,
		       v.created_at, v.updated_at,
		       COALESCE(vm.role, CASE WHEN v.user_id::text = $2 THEN 'OWNER' ELSE 'VIEWER' END) as role
		FROM vehicles v
		LEFT JOIN vehicle_members vm ON v.id = vm.vehicle_id AND vm.user_id::text = $2
		WHERE v.id::text = $1 AND (v.user_id::text = $2 OR vm.user_id IS NOT NULL);
	`
	var role string
	err := r.pool.QueryRow(ctx, query, id, userID).Scan(
		&v.ID, &v.UserID, &v.Name, &v.Vin, &v.TeslaMateCarID, &v.CurrentOdometer,
		&v.TeslaMateAPIURL, &v.TeslaMateAuthType, &v.TeslaMateAPIKeyEncrypted,
		&v.TeslaMateBasicUser, &v.TeslaMateBasicPassEnc,
		&v.PreTeslaMateKwh100km, &v.PreTeslaMateEurPerKwh,
		&v.CreatedAt, &v.UpdatedAt, &role,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get vehicle: %w", err)
	}
	v.Role = models.VehicleRole(role)
	sanitizeVehicleForRole(&v)
	return &v, nil
}

// GetVehicleByIDInternal returns vehicle without authentication or role filtering (used by background sync).
func (r *Repository) GetVehicleByIDInternal(ctx context.Context, id string) (*models.Vehicle, error) {
	var v models.Vehicle
	err := scanVehicle(r.pool.QueryRow(ctx, `SELECT `+vehicleColumns+` FROM vehicles WHERE id::text = $1;`, id), &v)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get vehicle: %w", err)
	}
	v.Role = models.RoleOwner
	return &v, nil
}

func (r *Repository) UpdateVehicle(ctx context.Context, v *models.Vehicle) error {
	query := `
		UPDATE vehicles
		SET name = $1, vin = $2, teslamate_car_id = $3, current_odometer = $4,
		    teslamate_api_url = $5, teslamate_auth_type = $6,
		    teslamate_api_key_encrypted = $7, teslamate_basic_user = $8,
		    teslamate_basic_pass_encrypted = $9,
		    pre_teslamate_kwh_100km = $10, pre_teslamate_eur_per_kwh = $11,
		    updated_at = NOW()
		WHERE id = $12;
	`
	tag, err := r.pool.Exec(ctx, query,
		v.Name, v.Vin, v.TeslaMateCarID, v.CurrentOdometer,
		v.TeslaMateAPIURL, v.TeslaMateAuthType, v.TeslaMateAPIKeyEncrypted,
		v.TeslaMateBasicUser, v.TeslaMateBasicPassEnc,
		v.PreTeslaMateKwh100km, v.PreTeslaMateEurPerKwh,
		v.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update vehicle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) UpdateVehiclePreTeslaMateEnergy(ctx context.Context, vehicleID, userID string, kwh100km, eurPerKwh *float64) error {
	query := `
		UPDATE vehicles
		SET pre_teslamate_kwh_100km = $1, pre_teslamate_eur_per_kwh = $2, updated_at = NOW()
		WHERE id::text = $3 AND (user_id::text = $4 OR EXISTS (
			SELECT 1 FROM vehicle_members vm WHERE vm.vehicle_id = vehicles.id AND vm.user_id::text = $4 AND vm.role IN ('OWNER', 'EDITOR')
		));
	`
	tag, err := r.pool.Exec(ctx, query, kwh100km, eurPerKwh, vehicleID, userID)
	if err != nil {
		return fmt.Errorf("failed to update pre-teslamate energy: %w", err)
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
	query := `
		DELETE FROM vehicles 
		WHERE id::text = $1 AND (user_id::text = $2 OR EXISTS (
			SELECT 1 FROM vehicle_members vm WHERE vm.vehicle_id = vehicles.id AND vm.user_id::text = $2 AND vm.role = 'OWNER'
		));
	`
	tag, err := r.pool.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("failed to delete vehicle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListVehicleMembers returns all members who have access to the vehicle.
func (r *Repository) ListVehicleMembers(ctx context.Context, vehicleID string) ([]models.VehicleMember, error) {
	query := `
		SELECT vm.vehicle_id, vm.user_id, vm.role, u.email, u.display_name, vm.created_at, vm.updated_at
		FROM vehicle_members vm
		JOIN users u ON vm.user_id = u.id
		WHERE vm.vehicle_id::text = $1
		ORDER BY 
			CASE vm.role WHEN 'OWNER' THEN 1 WHEN 'EDITOR' THEN 2 ELSE 3 END,
			vm.created_at ASC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("failed to list vehicle members: %w", err)
	}
	defer rows.Close()

	var members []models.VehicleMember
	for rows.Next() {
		var m models.VehicleMember
		var role string
		if err := rows.Scan(&m.VehicleID, &m.UserID, &role, &m.UserEmail, &m.DisplayName, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		m.Role = models.VehicleRole(role)
		members = append(members, m)
	}
	return members, rows.Err()
}

// AddVehicleMember associates an existing user to a vehicle by email with a specified role.
func (r *Repository) AddVehicleMember(ctx context.Context, vehicleID, userEmail string, role models.VehicleRole) (*models.VehicleMember, error) {
	if !role.IsValid() {
		return nil, fmt.Errorf("rôle invalide: %s", role)
	}
	var targetUser models.User
	err := r.pool.QueryRow(ctx, `SELECT id, email, display_name FROM users WHERE LOWER(email) = LOWER($1);`, userEmail).
		Scan(&targetUser.ID, &targetUser.Email, &targetUser.DisplayName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to find user by email: %w", err)
	}

	query := `
		INSERT INTO vehicle_members (vehicle_id, user_id, role)
		VALUES ($1, $2, $3)
		RETURNING created_at, updated_at;
	`
	var createdAt, updatedAt time.Time
	err = r.pool.QueryRow(ctx, query, vehicleID, targetUser.ID, string(role)).Scan(&createdAt, &updatedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "vehicle_members_pkey") {
			return nil, fmt.Errorf("cet utilisateur a déjà accès à ce véhicule")
		}
		return nil, fmt.Errorf("failed to add vehicle member: %w", err)
	}

	return &models.VehicleMember{
		VehicleID:   vehicleID,
		UserID:      targetUser.ID,
		Role:        role,
		UserEmail:   targetUser.Email,
		DisplayName: targetUser.DisplayName,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

// UpdateVehicleMemberRole updates the role of a vehicle member, preventing demotion of the last owner.
func (r *Repository) UpdateVehicleMemberRole(ctx context.Context, vehicleID, targetUserID string, newRole models.VehicleRole) error {
	if !newRole.IsValid() {
		return fmt.Errorf("rôle invalide: %s", newRole)
	}

	var currentRole string
	err := r.pool.QueryRow(ctx, `SELECT role FROM vehicle_members WHERE vehicle_id::text = $1 AND user_id::text = $2;`, vehicleID, targetUserID).Scan(&currentRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if models.VehicleRole(currentRole) == models.RoleOwner && newRole != models.RoleOwner {
		var ownerCount int
		if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vehicle_members WHERE vehicle_id::text = $1 AND role = 'OWNER';`, vehicleID).Scan(&ownerCount); err != nil {
			return err
		}
		if ownerCount <= 1 {
			return fmt.Errorf("impossible de rétrograder l'unique propriétaire du véhicule")
		}
	}

	query := `
		UPDATE vehicle_members
		SET role = $1, updated_at = NOW()
		WHERE vehicle_id::text = $2 AND user_id::text = $3;
	`
	tag, err := r.pool.Exec(ctx, query, string(newRole), vehicleID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to update vehicle member role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveVehicleMember removes a member's access to a vehicle, preventing removal of the last owner.
func (r *Repository) RemoveVehicleMember(ctx context.Context, vehicleID, targetUserID string) error {
	var currentRole string
	err := r.pool.QueryRow(ctx, `SELECT role FROM vehicle_members WHERE vehicle_id::text = $1 AND user_id::text = $2;`, vehicleID, targetUserID).Scan(&currentRole)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if models.VehicleRole(currentRole) == models.RoleOwner {
		var ownerCount int
		if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM vehicle_members WHERE vehicle_id::text = $1 AND role = 'OWNER';`, vehicleID).Scan(&ownerCount); err != nil {
			return err
		}
		if ownerCount <= 1 {
			return fmt.Errorf("impossible de retirer l'unique propriétaire du véhicule")
		}
	}

	tag, err := r.pool.Exec(ctx, `DELETE FROM vehicle_members WHERE vehicle_id::text = $1 AND user_id::text = $2;`, vehicleID, targetUserID)
	if err != nil {
		return fmt.Errorf("failed to remove vehicle member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetVehicleMemberRole returns the role of a user on a vehicle.
func (r *Repository) GetVehicleMemberRole(ctx context.Context, vehicleID, userID string) (models.VehicleRole, error) {
	var role string
	query := `
		SELECT COALESCE(vm.role, CASE WHEN v.user_id::text = $2 THEN 'OWNER' ELSE 'VIEWER' END)
		FROM vehicles v
		LEFT JOIN vehicle_members vm ON v.id = vm.vehicle_id AND vm.user_id::text = $2
		WHERE v.id::text = $1 AND (v.user_id::text = $2 OR vm.user_id IS NOT NULL);
	`
	err := r.pool.QueryRow(ctx, query, vehicleID, userID).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	return models.VehicleRole(role), nil
}

// ============================================================================
// Drives & Trip Groups
// ============================================================================

func (r *Repository) UpsertTeslaMateDrive(ctx context.Context, d *models.Drive) (bool, error) {
	query := `
		INSERT INTO drives (
			vehicle_id, teslamate_drive_id, start_time, end_time,
			start_odometer, end_odometer, distance_km, duration_min,
			speed_avg, speed_max, power_max, power_min, start_address, end_address, energy_consumed_kwh,
			consumption_kwh_100km, tags, is_manual
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, FALSE)
		ON CONFLICT (vehicle_id, teslamate_drive_id) DO UPDATE
		SET start_time = EXCLUDED.start_time,
		    end_time = EXCLUDED.end_time,
		    start_odometer = EXCLUDED.start_odometer,
		    end_odometer = EXCLUDED.end_odometer,
		    distance_km = EXCLUDED.distance_km,
		    duration_min = EXCLUDED.duration_min,
		    speed_avg = EXCLUDED.speed_avg,
		    speed_max = EXCLUDED.speed_max,
		    power_max = EXCLUDED.power_max,
		    power_min = EXCLUDED.power_min,
		    start_address = EXCLUDED.start_address,
		    end_address = EXCLUDED.end_address,
		    energy_consumed_kwh = EXCLUDED.energy_consumed_kwh,
		    consumption_kwh_100km = EXCLUDED.consumption_kwh_100km,
		    deleted_upstream_at = NULL,
		    updated_at = NOW()
		RETURNING id, (xmax = 0) AS is_inserted;
	`
	var isInserted bool
	err := r.pool.QueryRow(ctx, query,
		d.VehicleID, d.TeslaMateDriveID, d.StartTime, d.EndTime,
		d.StartOdometer, d.EndOdometer, d.DistanceKm, d.DurationMin,
		d.SpeedAvg, d.SpeedMax, d.PowerMax, d.PowerMin,
		d.StartAddress, d.EndAddress, d.EnergyConsumedKwh,
		d.ConsumptionKwh100km, d.Tags,
	).Scan(&d.ID, &isInserted)
	return isInserted, err
}

func (r *Repository) GetLatestTeslaMateDriveStartTime(ctx context.Context, vehicleID string) (*time.Time, error) {
	query := `
		SELECT start_time
		FROM drives
		WHERE vehicle_id = $1 AND teslamate_drive_id IS NOT NULL AND deleted_upstream_at IS NULL
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

// DriveFilter narrows drive listings.
type DriveFilter struct {
	Tag             string
	UnqualifiedOnly bool
	TripGroupID     string
	From            *time.Time
	To              *time.Time
	Query           string
}

// HighwayDrivePredicate matches drives likely to have used toll roads:
// either long and reasonably fast (distance >= 40 km, speed_avg >= 70 km/h),
// or over 20 km with a highway top speed (distance >= 20 km, speed_max > 125 km/h).
const HighwayDrivePredicate = `((drives.distance_km >= 40 AND COALESCE(drives.speed_avg, 0) >= 70) OR (drives.distance_km >= 20 AND COALESCE(drives.speed_max, 0) > 125))`

// Highway-like drives (long and fast) with no toll attached and no explicit "no toll" review.
const UnqualifiedDrivePredicate = HighwayDrivePredicate + `
	AND drives.toll_reviewed_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM drive_expenses de WHERE de.drive_id = drives.id)
	AND NOT EXISTS (
		SELECT 1 FROM drive_expenses de
		JOIN trip_group_drives tgd ON tgd.trip_group_id = de.trip_group_id
		WHERE tgd.drive_id = drives.id
	)
`

func (r *Repository) ListDrives(ctx context.Context, vehicleID string, filter DriveFilter, limit, offset int) ([]models.Drive, int, error) {
	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("vehicle_id = $%d", argIdx))
	args = append(args, vehicleID)
	argIdx++

	conditions = append(conditions, "deleted_upstream_at IS NULL")

	if filter.Tag != "" {
		conditions = append(conditions, fmt.Sprintf("$%d = ANY(tags)", argIdx))
		args = append(args, filter.Tag)
		argIdx++
	}

	if filter.UnqualifiedOnly {
		conditions = append(conditions, "("+UnqualifiedDrivePredicate+")")
	}

	if filter.TripGroupID != "" {
		conditions = append(conditions, fmt.Sprintf("id IN (SELECT drive_id FROM trip_group_drives WHERE trip_group_id::text = $%d::text)", argIdx))
		args = append(args, filter.TripGroupID)
		argIdx++
	}

	if filter.From != nil {
		conditions = append(conditions, fmt.Sprintf("start_time >= $%d", argIdx))
		args = append(args, *filter.From)
		argIdx++
	}

	if filter.To != nil {
		conditions = append(conditions, fmt.Sprintf("start_time <= $%d", argIdx))
		args = append(args, *filter.To)
		argIdx++
	}

	if strings.TrimSpace(filter.Query) != "" {
		pattern := "%" + strings.TrimSpace(filter.Query) + "%"
		conditions = append(conditions, fmt.Sprintf("(start_address ILIKE $%d OR end_address ILIKE $%d)", argIdx, argIdx))
		args = append(args, pattern)
		argIdx++
	}

	whereClause := strings.Join(conditions, " AND ")

	var total int
	countQuery := "SELECT COUNT(*) FROM drives WHERE " + whereClause
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, vehicle_id, teslamate_drive_id, start_time, end_time,
		       start_odometer, end_odometer, distance_km, duration_min,
		       speed_avg, speed_max, power_max, power_min, start_address, end_address, energy_consumed_kwh,
		       consumption_kwh_100km, tags, is_manual, toll_reviewed_at, created_at, updated_at
		FROM drives
		WHERE ` + whereClause + fmt.Sprintf(" ORDER BY start_time DESC LIMIT $%d OFFSET $%d;", argIdx, argIdx+1)

	queryArgs := append(args, limit, offset)
	rows, err := r.pool.Query(ctx, query, queryArgs...)
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
			&d.SpeedAvg, &d.SpeedMax, &d.PowerMax, &d.PowerMin,
			&d.StartAddress, &d.EndAddress, &d.EnergyConsumedKwh,
			&d.ConsumptionKwh100km, &d.Tags, &d.IsManual, &d.TollReviewedAt, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, d)
	}
	return list, total, rows.Err()
}

// CountUnqualifiedDrives counts highway-like drives still waiting for a toll qualification.
func (r *Repository) CountUnqualifiedDrives(ctx context.Context, vehicleID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM drives WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND `+UnqualifiedDrivePredicate, vehicleID).Scan(&count)
	return count, err
}

// SetDriveTollReviewed marks (or unmarks) a drive as explicitly reviewed without toll.
func (r *Repository) SetDriveTollReviewed(ctx context.Context, driveID, vehicleID string, reviewed bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE drives
		SET toll_reviewed_at = CASE WHEN $1 THEN NOW() ELSE NULL END, updated_at = NOW()
		WHERE id::text = $2 AND vehicle_id = $3;
	`, reviewed, driveID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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

	if err := insertTripGroup(ctx, tx, tg, driveIDs); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func insertTripGroup(ctx context.Context, tx pgx.Tx, tg *models.TripGroup, driveIDs []string) error {
	if err := ensureDrivesOwned(ctx, tx, tg.VehicleID, driveIDs); err != nil {
		return err
	}

	query := `
		INSERT INTO trip_groups (vehicle_id, name, notes)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at;
	`
	if err := tx.QueryRow(ctx, query, tg.VehicleID, tg.Name, tg.Notes).Scan(&tg.ID, &tg.CreatedAt, &tg.UpdatedAt); err != nil {
		return err
	}
	return linkTripGroupDrives(ctx, tx, tg.ID, driveIDs)
}

// linkTripGroupDrives links drives to a trip group, ordered chronologically.
func linkTripGroupDrives(ctx context.Context, tx pgx.Tx, tripGroupID string, driveIDs []string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO trip_group_drives (trip_group_id, drive_id, order_index)
		SELECT $1, d.id, ROW_NUMBER() OVER (ORDER BY d.start_time) - 1
		FROM drives d
		WHERE d.id::text = ANY($2::text[]) AND d.deleted_upstream_at IS NULL;
	`, tripGroupID, uniqueStrings(driveIDs))
	if err != nil {
		return fmt.Errorf("failed to link drives to trip group: %w", err)
	}
	return nil
}

func (r *Repository) ListTripGroups(ctx context.Context, vehicleID string) ([]models.TripGroup, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT tg.id, tg.vehicle_id, tg.name, tg.notes, tg.created_at, tg.updated_at,
		       ARRAY(SELECT tgd.drive_id::text FROM trip_group_drives tgd
		             JOIN drives d ON d.id = tgd.drive_id AND d.deleted_upstream_at IS NULL
		             WHERE tgd.trip_group_id = tg.id ORDER BY d.start_time),
		       COALESCE(stats.km, 0), stats.first_start, stats.last_end,
		       COALESCE((SELECT SUM(`+AmountEURExpr+`) FROM drive_expenses e WHERE e.trip_group_id = tg.id), 0),
		       (SELECT COUNT(*) FROM drive_expenses e WHERE e.trip_group_id = tg.id),
		       (SELECT COUNT(*) FROM carpool_trips c WHERE c.trip_group_id = tg.id)
		FROM trip_groups tg
		LEFT JOIN LATERAL (
			SELECT SUM(d.distance_km) AS km, MIN(d.start_time) AS first_start, MAX(d.end_time) AS last_end
			FROM trip_group_drives tgd
			JOIN drives d ON d.id = tgd.drive_id AND d.deleted_upstream_at IS NULL
			WHERE tgd.trip_group_id = tg.id
		) stats ON TRUE
		WHERE tg.vehicle_id = $1
		ORDER BY COALESCE(stats.first_start, tg.created_at) DESC;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.TripGroup
	for rows.Next() {
		var tg models.TripGroup
		if err := rows.Scan(&tg.ID, &tg.VehicleID, &tg.Name, &tg.Notes, &tg.CreatedAt, &tg.UpdatedAt,
			&tg.DriveIDs, &tg.DistanceKm, &tg.StartTime, &tg.EndTime, &tg.ExpensesTotal, &tg.ExpenseCount, &tg.CarpoolCount); err != nil {
			return nil, err
		}
		list = append(list, tg)
	}
	return list, rows.Err()
}

// UpdateTripGroup renames a trip group and, when driveIDs is not nil, replaces its drives.
func (r *Repository) UpdateTripGroup(ctx context.Context, tg *models.TripGroup, driveIDs []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE trip_groups SET name = $1, notes = $2, updated_at = NOW()
		WHERE id::text = $3 AND vehicle_id = $4;
	`, tg.Name, tg.Notes, tg.ID, tg.VehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if driveIDs != nil {
		if len(uniqueStrings(driveIDs)) == 0 {
			return validationErrorf("un voyage doit contenir au moins un trajet")
		}
		if err := ensureDrivesOwned(ctx, tx, tg.VehicleID, driveIDs); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM trip_group_drives WHERE trip_group_id::text = $1;`, tg.ID); err != nil {
			return err
		}
		if err := linkTripGroupDrives(ctx, tx, tg.ID, driveIDs); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// DeleteTripGroup deletes a trip group. Its expenses are kept (no longer linked to drives) unless deleteExpenses is set.
func (r *Repository) DeleteTripGroup(ctx context.Context, vehicleID, tripGroupID string, deleteExpenses bool) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := ensureTripGroupOwned(ctx, tx, vehicleID, tripGroupID); err != nil {
		return ErrNotFound
	}
	if deleteExpenses {
		if _, err := tx.Exec(ctx, `DELETE FROM drive_expenses WHERE trip_group_id::text = $1 AND vehicle_id = $2;`, tripGroupID, vehicleID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM trip_groups WHERE id::text = $1 AND vehicle_id = $2;`, tripGroupID, vehicleID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ============================================================================
// Drive Expenses (Tolls, Parking)
// ============================================================================

// AmountEURExpr converts an expense amount to EUR; NULL when a foreign amount has no conversion rate.
const AmountEURExpr = `(CASE WHEN currency = 'EUR' THEN amount ELSE amount * fx_rate END)`

// SaveDriveExpense creates (exp.ID empty) or updates a drive expense in a single transaction.
// When groupDriveIDs contains several drives, the expense is attached to a trip group: the group
// previously dedicated to this expense is resynchronized, otherwise a new group named groupName is created.
func (r *Repository) SaveDriveExpense(ctx context.Context, exp *models.DriveExpense, groupDriveIDs []string, groupName string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var previousGroupID *string
	if exp.ID != "" {
		err := tx.QueryRow(ctx, `
			SELECT trip_group_id FROM drive_expenses WHERE id::text = $1 AND vehicle_id = $2 FOR UPDATE;
		`, exp.ID, exp.VehicleID).Scan(&previousGroupID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
	}

	ids := uniqueStrings(groupDriveIDs)
	switch {
	case len(ids) > 1:
		if err := ensureDrivesOwned(ctx, tx, exp.VehicleID, ids); err != nil {
			return err
		}
		groupID, err := r.reusableExpenseGroup(ctx, tx, exp, previousGroupID)
		if err != nil {
			return err
		}
		if groupID != "" {
			if _, err := tx.Exec(ctx, `DELETE FROM trip_group_drives WHERE trip_group_id = $1;`, groupID); err != nil {
				return err
			}
			if err := linkTripGroupDrives(ctx, tx, groupID, ids); err != nil {
				return err
			}
		} else {
			tg := &models.TripGroup{VehicleID: exp.VehicleID, Name: groupName, Notes: exp.Notes}
			if err := insertTripGroup(ctx, tx, tg, ids); err != nil {
				return err
			}
			groupID = tg.ID
		}
		exp.TripGroupID = &groupID
		exp.DriveID = nil
	case len(ids) == 1:
		exp.DriveID = &ids[0]
		exp.TripGroupID = nil
	}

	if exp.DriveID != nil && *exp.DriveID == "" {
		exp.DriveID = nil
	}
	if exp.TripGroupID != nil && *exp.TripGroupID == "" {
		exp.TripGroupID = nil
	}
	if exp.TripGroupID != nil {
		exp.DriveID = nil
		if err := ensureTripGroupOwned(ctx, tx, exp.VehicleID, *exp.TripGroupID); err != nil {
			return err
		}
	}
	if exp.DriveID != nil {
		if err := ensureDrivesOwned(ctx, tx, exp.VehicleID, []string{*exp.DriveID}); err != nil {
			return err
		}
	}

	if exp.ID == "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO drive_expenses (vehicle_id, trip_group_id, drive_id, type, amount, currency, fx_rate, date, notes, document_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
			RETURNING id, created_at;
		`, exp.VehicleID, exp.TripGroupID, exp.DriveID, exp.Type,
			exp.Amount, exp.Currency, exp.FxRate, exp.Date, exp.Notes, exp.DocumentID,
		).Scan(&exp.ID, &exp.CreatedAt)
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE drive_expenses
			SET trip_group_id = $1, drive_id = $2, type = $3, amount = $4,
			    currency = $5, fx_rate = $6, date = $7, notes = $8, document_id = $9
			WHERE id::text = $10 AND vehicle_id = $11
			RETURNING created_at;
		`, exp.TripGroupID, exp.DriveID, exp.Type, exp.Amount,
			exp.Currency, exp.FxRate, exp.Date, exp.Notes, exp.DocumentID,
			exp.ID, exp.VehicleID,
		).Scan(&exp.CreatedAt)
	}
	if err != nil {
		return err
	}

	if exp.DocumentID != nil {
		_ = tx.QueryRow(ctx, `SELECT filename FROM expense_documents WHERE id = $1;`, *exp.DocumentID).Scan(&exp.DocumentFilename)
	}

	return tx.Commit(ctx)
}

// reusableExpenseGroup returns the expense's current trip group when no other expense or carpool uses it.
func (r *Repository) reusableExpenseGroup(ctx context.Context, tx pgx.Tx, exp *models.DriveExpense, previousGroupID *string) (string, error) {
	if previousGroupID == nil {
		return "", nil
	}
	var shared bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM drive_expenses WHERE trip_group_id = $1 AND id::text <> $2)
		    OR EXISTS(SELECT 1 FROM carpool_trips WHERE trip_group_id = $1);
	`, *previousGroupID, exp.ID).Scan(&shared)
	if err != nil {
		return "", err
	}
	if shared {
		return "", nil
	}
	return *previousGroupID, nil
}

func (r *Repository) ListDriveExpenses(ctx context.Context, vehicleID string) ([]models.DriveExpense, error) {
	query := `
		SELECT
			e.id, e.vehicle_id, e.trip_group_id, tg.name,
			ARRAY(SELECT tgd.drive_id::text FROM trip_group_drives tgd WHERE tgd.trip_group_id = e.trip_group_id ORDER BY tgd.order_index),
			e.drive_id,
			CASE
				WHEN d.id IS NOT NULL THEN COALESCE(NULLIF(d.start_address, ''), 'Départ') || ' → ' || COALESCE(NULLIF(d.end_address, ''), 'Arrivée')
				ELSE NULL
			END,
			e.type, e.amount, e.currency, e.fx_rate, e.date, e.notes,
			e.document_id, doc.filename,
			e.created_at
		FROM drive_expenses e
		LEFT JOIN drives d ON e.drive_id = d.id AND d.vehicle_id = e.vehicle_id
		LEFT JOIN trip_groups tg ON e.trip_group_id = tg.id AND tg.vehicle_id = e.vehicle_id
		LEFT JOIN expense_documents doc ON e.document_id = doc.id
		WHERE e.vehicle_id = $1
		ORDER BY e.date DESC;
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
			&e.ID, &e.VehicleID, &e.TripGroupID, &e.TripGroupName, &e.TripGroupDriveIDs,
			&e.DriveID, &e.DriveTitle, &e.Type,
			&e.Amount, &e.Currency, &e.FxRate, &e.Date, &e.Notes,
			&e.DocumentID, &e.DocumentFilename,
			&e.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, e)
	}
	return list, rows.Err()
}

func (r *Repository) DeleteDriveExpense(ctx context.Context, vehicleID, expenseID string) error {
	query := `DELETE FROM drive_expenses WHERE id::text = $1 AND vehicle_id = $2;`
	cmdTag, err := r.pool.Exec(ctx, query, expenseID, vehicleID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Tires, Wear Logs & Rotations
// ============================================================================

const tireColumns = `
	id, vehicle_id, brand, model, dimension, season,
	purchase_date, purchase_price, current_position,
	initial_depth_mm, min_legal_depth_mm, dot_code, is_archived,
	mounted_odometer, initial_distance_km, accumulated_distance_km, estimated_lifespan_km,
	created_at, updated_at
`

func scanTire(row pgx.Row, t *models.Tire) error {
	return row.Scan(
		&t.ID, &t.VehicleID, &t.Brand, &t.Model, &t.Dimension, &t.Season,
		&t.PurchaseDate, &t.PurchasePrice, &t.CurrentPosition,
		&t.InitialDepthMm, &t.MinLegalDepthMm, &t.DotCode, &t.IsArchived,
		&t.MountedOdometer, &t.InitialDistanceKm, &t.AccumulatedDistanceKm, &t.EstimatedLifespanKm,
		&t.CreatedAt, &t.UpdatedAt,
	)
}

func isMountedPosition(pos models.TirePosition) bool {
	switch pos {
	case models.TirePosFL, models.TirePosFR, models.TirePosRL, models.TirePosRR:
		return true
	}
	return false
}

func isValidTirePosition(pos models.TirePosition) bool {
	return isMountedPosition(pos) || pos == models.TirePosStorage || pos == models.TirePosDisposed
}

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

func (r *Repository) CreateMaintenanceExpense(ctx context.Context, m *models.MaintenanceExpense) error {
	mode := m.AmortizationMode
	if mode == "" {
		mode = "NONE"
	}
	m.AmortizationMode = mode

	query := `
		INSERT INTO maintenance_expenses (
			vehicle_id, category, amount, currency, fx_rate, date,
			odometer, is_recurring, recurrence_interval_months, recurrence_end_date, description,
			amortization_mode, coverage_km, coverage_months, closes_maintenance_id, document_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING id, created_at, updated_at;
	`
	err := r.pool.QueryRow(ctx, query,
		m.VehicleID, m.Category, m.Amount, m.Currency, m.FxRate, m.Date,
		m.Odometer, m.IsRecurring, m.RecurrenceIntervalMonths, m.RecurrenceEndDate, m.Description,
		m.AmortizationMode, m.CoverageKm, m.CoverageMonths, m.ClosesMaintenanceID, m.DocumentID,
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return err
	}

	if m.DocumentID != nil {
		_ = r.pool.QueryRow(ctx, `SELECT filename FROM expense_documents WHERE id = $1;`, *m.DocumentID).Scan(&m.DocumentFilename)
	}

	return nil
}

func (r *Repository) ListMaintenanceExpenses(ctx context.Context, vehicleID string) ([]models.MaintenanceExpense, error) {
	query := `
		SELECT m.id, m.vehicle_id, m.category, m.amount, m.currency, m.fx_rate, m.date,
		       m.odometer, m.is_recurring, m.recurrence_interval_months, m.recurrence_end_date, m.description,
		       m.amortization_mode, m.coverage_km, m.coverage_months, m.closes_maintenance_id,
		       m.document_id, doc.filename,
		       m.created_at, m.updated_at
		FROM maintenance_expenses m
		LEFT JOIN expense_documents doc ON m.document_id = doc.id
		WHERE m.vehicle_id = $1
		ORDER BY m.date DESC;
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
			&m.ID, &m.VehicleID, &m.Category, &m.Amount, &m.Currency, &m.FxRate, &m.Date,
			&m.Odometer, &m.IsRecurring, &m.RecurrenceIntervalMonths, &m.RecurrenceEndDate, &m.Description,
			&m.AmortizationMode, &m.CoverageKm, &m.CoverageMonths, &m.ClosesMaintenanceID,
			&m.DocumentID, &m.DocumentFilename,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (r *Repository) UpdateMaintenanceExpense(ctx context.Context, m *models.MaintenanceExpense) error {
	mode := m.AmortizationMode
	if mode == "" {
		mode = "NONE"
	}
	m.AmortizationMode = mode

	query := `
		UPDATE maintenance_expenses
		SET category = $1,
		    amount = $2,
		    currency = $3,
		    fx_rate = $4,
		    date = $5,
		    odometer = $6,
		    is_recurring = $7,
		    recurrence_interval_months = $8,
		    recurrence_end_date = $9,
		    description = $10,
		    amortization_mode = $11,
		    coverage_km = $12,
		    coverage_months = $13,
		    closes_maintenance_id = $14,
		    document_id = $15,
		    updated_at = NOW()
		WHERE id::text = $16 AND vehicle_id = $17
		RETURNING created_at, updated_at;
	`
	err := r.pool.QueryRow(ctx, query,
		m.Category, m.Amount, m.Currency, m.FxRate, m.Date,
		m.Odometer, m.IsRecurring, m.RecurrenceIntervalMonths, m.RecurrenceEndDate, m.Description,
		m.AmortizationMode, m.CoverageKm, m.CoverageMonths, m.ClosesMaintenanceID, m.DocumentID,
		m.ID, m.VehicleID,
	).Scan(&m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if m.DocumentID != nil {
		_ = r.pool.QueryRow(ctx, `SELECT filename FROM expense_documents WHERE id = $1;`, *m.DocumentID).Scan(&m.DocumentFilename)
	}

	return nil
}

func (r *Repository) DeleteMaintenanceExpense(ctx context.Context, vehicleID, maintenanceID string) error {
	query := `DELETE FROM maintenance_expenses WHERE id::text = $1 AND vehicle_id = $2;`
	cmdTag, err := r.pool.Exec(ctx, query, maintenanceID, vehicleID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetOdometerAtDate resolves the vehicle odometer at or near a specific timestamp
// using TeslaMate drives, falling back to current_odometer.
func (r *Repository) GetOdometerAtDate(ctx context.Context, vehicleID string, at time.Time) (float64, string, error) {
	query := `
		SELECT COALESCE(
			(SELECT end_odometer FROM drives
			 WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND end_time <= $2 AND end_odometer > 0
			 ORDER BY end_time DESC LIMIT 1),
			(SELECT start_odometer FROM drives
			 WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL AND start_time >= $2 AND start_odometer > 0
			 ORDER BY start_time ASC LIMIT 1),
			(SELECT current_odometer FROM vehicles WHERE id = $1),
			0
		);
	`
	var odo float64
	err := r.pool.QueryRow(ctx, query, vehicleID, at).Scan(&odo)
	if err != nil {
		return 0, "unknown", err
	}
	return odo, "teslamate", nil
}


// ============================================================================
// Charges
// ============================================================================

// UpsertTeslaMateCharge inserts or refreshes a TeslaMate charge. A cost entered manually is never overwritten.
func (r *Repository) UpsertTeslaMateCharge(ctx context.Context, c *models.ChargeLog) (bool, error) {
	query := `
		INSERT INTO charge_logs (
			vehicle_id, teslamate_charge_id, date, end_date,
			address, kwh_added, kwh_used, cost, cost_source, currency, odometer, is_manual
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'TESLAMATE', $9, $10, FALSE)
		ON CONFLICT (vehicle_id, teslamate_charge_id) DO UPDATE
		SET date = EXCLUDED.date,
		    end_date = EXCLUDED.end_date,
		    address = EXCLUDED.address,
		    kwh_added = EXCLUDED.kwh_added,
		    kwh_used = EXCLUDED.kwh_used,
		    cost = CASE WHEN charge_logs.cost_source = 'MANUAL' THEN charge_logs.cost ELSE EXCLUDED.cost END,
		    currency = CASE WHEN charge_logs.cost_source = 'MANUAL' THEN charge_logs.currency ELSE EXCLUDED.currency END,
		    odometer = EXCLUDED.odometer,
		    deleted_upstream_at = NULL
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
		WHERE vehicle_id = $1 AND teslamate_charge_id IS NOT NULL AND deleted_upstream_at IS NULL
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

const chargeSelectColumns = `
	c.id, c.vehicle_id, c.teslamate_charge_id, c.date, c.end_date,
	c.address, c.kwh_added, c.kwh_used, c.cost, c.cost_source, c.currency, c.fx_rate, c.odometer, c.is_manual, c.notes,
	c.document_id, doc.filename, c.created_at
`

func scanCharge(row pgx.Row, c *models.ChargeLog) error {
	return row.Scan(
		&c.ID, &c.VehicleID, &c.TeslaMateChargeID, &c.Date, &c.EndDate,
		&c.Address, &c.KwhAdded, &c.KwhUsed, &c.Cost, &c.CostSource, &c.Currency, &c.FxRate, &c.Odometer,
		&c.IsManual, &c.Notes,
		&c.DocumentID, &c.DocumentFilename,
		&c.CreatedAt,
	)
}

// ListCharges lists charges; missingCostOnly restricts to charges whose cost is still unknown.
func (r *Repository) ListCharges(ctx context.Context, vehicleID string, missingCostOnly bool, limit, offset int) ([]models.ChargeLog, int, error) {
	whereCount := `vehicle_id = $1 AND deleted_upstream_at IS NULL AND (NOT $2 OR cost IS NULL)`
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM charge_logs WHERE `+whereCount, vehicleID, missingCostOnly).Scan(&total); err != nil {
		return nil, 0, err
	}

	whereSelect := `c.vehicle_id = $1 AND c.deleted_upstream_at IS NULL AND (NOT $2 OR c.cost IS NULL)`
	rows, err := r.pool.Query(ctx, `
		SELECT `+chargeSelectColumns+`
		FROM charge_logs c
		LEFT JOIN expense_documents doc ON c.document_id = doc.id
		WHERE `+whereSelect+`
		ORDER BY c.date DESC
		LIMIT $3 OFFSET $4;
	`, vehicleID, missingCostOnly, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []models.ChargeLog
	for rows.Next() {
		var c models.ChargeLog
		if err := scanCharge(rows, &c); err != nil {
			return nil, 0, err
		}
		list = append(list, c)
	}
	return list, total, rows.Err()
}

// CountChargesWithoutCost counts charges with an unknown cost.
func (r *Repository) CountChargesWithoutCost(ctx context.Context, vehicleID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM charge_logs WHERE vehicle_id = $1 AND cost IS NULL AND deleted_upstream_at IS NULL;`, vehicleID).Scan(&count)
	return count, err
}

// CreateManualCharge records a charge that TeslaMate did not capture.
func (r *Repository) CreateManualCharge(ctx context.Context, c *models.ChargeLog) error {
	c.IsManual = true
	c.CostSource = "MANUAL"
	err := r.pool.QueryRow(ctx, `
		INSERT INTO charge_logs (
			vehicle_id, date, end_date, address, kwh_added, cost, cost_source,
			currency, fx_rate, odometer, is_manual, notes, document_id
		) VALUES ($1, $2, $3, $4, $5, $6, 'MANUAL', $7, $8, $9, TRUE, $10, $11)
		RETURNING id, created_at;
	`,
		c.VehicleID, c.Date, c.EndDate, c.Address, c.KwhAdded, c.Cost,
		c.Currency, c.FxRate, c.Odometer, c.Notes, c.DocumentID,
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return err
	}

	if c.DocumentID != nil {
		_ = r.pool.QueryRow(ctx, `SELECT filename FROM expense_documents WHERE id = $1;`, *c.DocumentID).Scan(&c.DocumentFilename)
	}

	return nil
}

// UpdateCharge updates a charge. TeslaMate charges only accept cost corrections (protected from resyncs);
// manual charges are fully editable.
func (r *Repository) UpdateCharge(ctx context.Context, c *models.ChargeLog) error {
	var existing models.ChargeLog
	err := scanCharge(r.pool.QueryRow(ctx, `
		SELECT `+chargeSelectColumns+`
		FROM charge_logs c
		LEFT JOIN expense_documents doc ON c.document_id = doc.id
		WHERE c.id::text = $1 AND c.vehicle_id = $2;
	`, c.ID, c.VehicleID), &existing)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	}

	if !existing.IsManual {
		c.Date, c.EndDate, c.Address = existing.Date, existing.EndDate, existing.Address
		c.KwhAdded, c.Odometer = existing.KwhAdded, existing.Odometer
	}

	cmdTag, err := r.pool.Exec(ctx, `
		UPDATE charge_logs
		SET date = $1, end_date = $2, address = $3, kwh_added = $4, odometer = $5,
		    cost = $6, cost_source = 'MANUAL', currency = $7, fx_rate = $8, notes = $9,
		    document_id = $10
		WHERE id::text = $11 AND vehicle_id = $12;
	`,
		c.Date, c.EndDate, c.Address, c.KwhAdded, c.Odometer,
		c.Cost, c.Currency, c.FxRate, c.Notes, c.DocumentID,
		c.ID, c.VehicleID,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return scanCharge(r.pool.QueryRow(ctx, `
		SELECT `+chargeSelectColumns+`
		FROM charge_logs c
		LEFT JOIN expense_documents doc ON c.document_id = doc.id
		WHERE c.id::text = $1 AND c.vehicle_id = $2;
	`, c.ID, c.VehicleID), c)
}

// DeleteManualCharge deletes a manually entered charge (TeslaMate charges would come back on next sync).
func (r *Repository) DeleteManualCharge(ctx context.Context, vehicleID, chargeID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM charge_logs WHERE id::text = $1 AND vehicle_id = $2 AND is_manual = TRUE;`, chargeID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Sync State
// ============================================================================

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

// CreateCarpoolTrip stores a new carpool trip with its legs and passengers.
func (r *Repository) CreateCarpoolTrip(ctx context.Context, trip *models.CarpoolTrip, legs []models.CarpoolLeg, passengers []models.CarpoolPassenger) error {
	trip.ID = ""
	return r.saveCarpoolTrip(ctx, trip, legs, passengers)
}

// UpdateCarpoolTrip replaces a carpool trip, its legs and its passengers.
func (r *Repository) UpdateCarpoolTrip(ctx context.Context, trip *models.CarpoolTrip, legs []models.CarpoolLeg, passengers []models.CarpoolPassenger) error {
	if trip.ID == "" {
		return ErrNotFound
	}
	return r.saveCarpoolTrip(ctx, trip, legs, passengers)
}

// saveCarpoolTrip writes a trip atomically. Trip totals are the sums of its legs; passengers' stops must
// reference existing stops (0..len(legs)).
func (r *Repository) saveCarpoolTrip(ctx context.Context, trip *models.CarpoolTrip, legs []models.CarpoolLeg, passengers []models.CarpoolPassenger) error {
	if len(legs) == 0 {
		return validationErrorf("un covoiturage doit comporter au moins une étape")
	}
	for _, p := range passengers {
		if p.BoardStopIndex < 0 || p.AlightStopIndex <= p.BoardStopIndex || p.AlightStopIndex > len(legs) {
			return validationErrorf("arrêts de montée et de descente invalides pour %s", p.PassengerName)
		}
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := ensureCarpoolLinksOwned(ctx, tx, trip); err != nil {
		return err
	}
	var driveIDs []string
	for _, l := range legs {
		if l.DriveID != nil && *l.DriveID != "" {
			driveIDs = append(driveIDs, *l.DriveID)
		}
	}
	if err := ensureDrivesOwned(ctx, tx, trip.VehicleID, driveIDs); err != nil {
		return err
	}

	trip.DistanceKm, trip.ElectricityCost, trip.TollsCost, trip.TiresCost = 0, 0, 0, 0
	trip.MaintenanceCost, trip.InsuranceCost, trip.OtherCost, trip.TotalRevenue = 0, 0, 0, 0
	for _, l := range legs {
		trip.DistanceKm += l.DistanceKm
		trip.ElectricityCost += l.ElectricityCost
		trip.TollsCost += l.TollsCost
		trip.TiresCost += l.TiresCost
		trip.MaintenanceCost += l.MaintenanceCost
		trip.InsuranceCost += l.InsuranceCost
		trip.OtherCost += l.OtherCost
	}
	for _, p := range passengers {
		trip.TotalRevenue += p.AmountPaid
	}
	trip.DistanceKm = math.Round(trip.DistanceKm*100) / 100
	trip.TotalCost = trip.ElectricityCost + trip.TollsCost + trip.TiresCost + trip.MaintenanceCost + trip.InsuranceCost + trip.OtherCost
	trip.NetCost = trip.TotalCost - trip.TotalRevenue

	if trip.ID == "" {
		err = tx.QueryRow(ctx, `
			INSERT INTO carpool_trips (
				vehicle_id, drive_id, trip_group_id, title, date, distance_km,
				electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost,
				total_cost, total_revenue, net_cost, notes
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
			RETURNING id, created_at, updated_at;
		`,
			trip.VehicleID, trip.DriveID, trip.TripGroupID, trip.Title, trip.Date, trip.DistanceKm,
			trip.ElectricityCost, trip.TollsCost, trip.TiresCost, trip.MaintenanceCost, trip.InsuranceCost, trip.OtherCost,
			trip.TotalCost, trip.TotalRevenue, trip.NetCost, trip.Notes,
		).Scan(&trip.ID, &trip.CreatedAt, &trip.UpdatedAt)
	} else {
		err = tx.QueryRow(ctx, `
			UPDATE carpool_trips
			SET drive_id = $1, trip_group_id = $2, title = $3, date = $4, distance_km = $5,
			    electricity_cost = $6, tolls_cost = $7, tires_cost = $8, maintenance_cost = $9,
			    insurance_cost = $10, other_cost = $11, total_cost = $12, total_revenue = $13,
			    net_cost = $14, notes = $15, updated_at = NOW()
			WHERE id::text = $16 AND vehicle_id = $17
			RETURNING created_at, updated_at;
		`,
			trip.DriveID, trip.TripGroupID, trip.Title, trip.Date, trip.DistanceKm,
			trip.ElectricityCost, trip.TollsCost, trip.TiresCost, trip.MaintenanceCost,
			trip.InsuranceCost, trip.OtherCost, trip.TotalCost, trip.TotalRevenue,
			trip.NetCost, trip.Notes, trip.ID, trip.VehicleID,
		).Scan(&trip.CreatedAt, &trip.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
	}
	if err != nil {
		return fmt.Errorf("failed to save carpool trip: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM carpool_legs WHERE carpool_trip_id = $1;`, trip.ID); err != nil {
		return fmt.Errorf("failed to clear legs: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM carpool_passengers WHERE carpool_trip_id = $1;`, trip.ID); err != nil {
		return fmt.Errorf("failed to clear passengers: %w", err)
	}

	for i := range legs {
		l := &legs[i]
		l.CarpoolTripID, l.OrderIndex = trip.ID, i
		if err := tx.QueryRow(ctx, `
			INSERT INTO carpool_legs (
				carpool_trip_id, order_index, drive_id, start_label, end_label, distance_km,
				electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			RETURNING id;
		`, l.CarpoolTripID, l.OrderIndex, l.DriveID, l.StartLabel, l.EndLabel, l.DistanceKm,
			l.ElectricityCost, l.TollsCost, l.TiresCost, l.MaintenanceCost, l.InsuranceCost, l.OtherCost,
		).Scan(&l.ID); err != nil {
			return fmt.Errorf("failed to insert carpool leg: %w", err)
		}
	}

	for i := range passengers {
		p := &passengers[i]
		p.CarpoolTripID = trip.ID
		if err := tx.QueryRow(ctx, `
			INSERT INTO carpool_passengers (
				carpool_trip_id, passenger_name, origin, destination, seats, amount_paid, notes,
				board_stop_index, alight_stop_index
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			RETURNING id, created_at;
		`, p.CarpoolTripID, p.PassengerName, p.Origin, p.Destination, p.Seats, p.AmountPaid, p.Notes,
			p.BoardStopIndex, p.AlightStopIndex,
		).Scan(&p.ID, &p.CreatedAt); err != nil {
			return fmt.Errorf("failed to insert carpool passenger: %w", err)
		}
	}

	return tx.Commit(ctx)
}

func ensureCarpoolLinksOwned(ctx context.Context, tx pgx.Tx, trip *models.CarpoolTrip) error {
	if trip.DriveID != nil && *trip.DriveID == "" {
		trip.DriveID = nil
	}
	if trip.TripGroupID != nil && *trip.TripGroupID == "" {
		trip.TripGroupID = nil
	}
	if trip.DriveID != nil {
		if err := ensureDrivesOwned(ctx, tx, trip.VehicleID, []string{*trip.DriveID}); err != nil {
			return err
		}
	}
	if trip.TripGroupID != nil {
		if err := ensureTripGroupOwned(ctx, tx, trip.VehicleID, *trip.TripGroupID); err != nil {
			return err
		}
	}
	return nil
}

const carpoolTripColumns = `
	id, vehicle_id, drive_id, trip_group_id, title, date, distance_km,
	electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost,
	total_cost, total_revenue, net_cost, notes, created_at, updated_at
`

func scanCarpoolTrip(row pgx.Row, t *models.CarpoolTripWithPassengers) error {
	return row.Scan(
		&t.ID, &t.VehicleID, &t.DriveID, &t.TripGroupID, &t.Title, &t.Date, &t.DistanceKm,
		&t.ElectricityCost, &t.TollsCost, &t.TiresCost, &t.MaintenanceCost, &t.InsuranceCost, &t.OtherCost,
		&t.TotalCost, &t.TotalRevenue, &t.NetCost, &t.Notes, &t.CreatedAt, &t.UpdatedAt,
	)
}

// ListCarpoolTrips returns the carpool trips of a vehicle with their legs and passengers.
func (r *Repository) ListCarpoolTrips(ctx context.Context, vehicleID string) ([]models.CarpoolTripWithPassengers, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+carpoolTripColumns+` FROM carpool_trips WHERE vehicle_id = $1 ORDER BY date DESC, created_at DESC;`, vehicleID)
	if err != nil {
		return nil, fmt.Errorf("failed to list carpool trips: %w", err)
	}
	trips := []models.CarpoolTripWithPassengers{}
	for rows.Next() {
		var t models.CarpoolTripWithPassengers
		if err := scanCarpoolTrip(rows, &t); err != nil {
			rows.Close()
			return nil, err
		}
		trips = append(trips, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.loadCarpoolDetails(ctx, trips); err != nil {
		return nil, err
	}
	return trips, nil
}

func (r *Repository) GetCarpoolTrip(ctx context.Context, id, vehicleID string) (*models.CarpoolTripWithPassengers, error) {
	var t models.CarpoolTripWithPassengers
	err := scanCarpoolTrip(r.pool.QueryRow(ctx, `SELECT `+carpoolTripColumns+` FROM carpool_trips WHERE id::text = $1 AND vehicle_id = $2;`, id, vehicleID), &t)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get carpool trip: %w", err)
	}
	trips := []models.CarpoolTripWithPassengers{t}
	if err := r.loadCarpoolDetails(ctx, trips); err != nil {
		return nil, err
	}
	return &trips[0], nil
}

// loadCarpoolDetails loads legs and passengers of several trips in two queries.
func (r *Repository) loadCarpoolDetails(ctx context.Context, trips []models.CarpoolTripWithPassengers) error {
	if len(trips) == 0 {
		return nil
	}
	ids := make([]string, len(trips))
	index := make(map[string]int, len(trips))
	for i := range trips {
		ids[i] = trips[i].ID
		index[trips[i].ID] = i
		trips[i].Legs = []models.CarpoolLeg{}
		trips[i].Passengers = []models.CarpoolPassenger{}
	}

	legRows, err := r.pool.Query(ctx, `
		SELECT id, carpool_trip_id, order_index, drive_id, start_label, end_label, distance_km,
		       electricity_cost, tolls_cost, tires_cost, maintenance_cost, insurance_cost, other_cost
		FROM carpool_legs
		WHERE carpool_trip_id::text = ANY($1::text[])
		ORDER BY carpool_trip_id, order_index;
	`, ids)
	if err != nil {
		return fmt.Errorf("failed to load carpool legs: %w", err)
	}
	for legRows.Next() {
		var l models.CarpoolLeg
		if err := legRows.Scan(&l.ID, &l.CarpoolTripID, &l.OrderIndex, &l.DriveID, &l.StartLabel, &l.EndLabel, &l.DistanceKm,
			&l.ElectricityCost, &l.TollsCost, &l.TiresCost, &l.MaintenanceCost, &l.InsuranceCost, &l.OtherCost); err != nil {
			legRows.Close()
			return err
		}
		t := &trips[index[l.CarpoolTripID]]
		t.Legs = append(t.Legs, l)
	}
	legRows.Close()
	if err := legRows.Err(); err != nil {
		return err
	}

	pRows, err := r.pool.Query(ctx, `
		SELECT id, carpool_trip_id, passenger_name, origin, destination, seats, amount_paid, notes, created_at,
		       board_stop_index, alight_stop_index
		FROM carpool_passengers
		WHERE carpool_trip_id::text = ANY($1::text[])
		ORDER BY created_at ASC;
	`, ids)
	if err != nil {
		return fmt.Errorf("failed to load carpool passengers: %w", err)
	}
	defer pRows.Close()
	for pRows.Next() {
		var p models.CarpoolPassenger
		if err := pRows.Scan(&p.ID, &p.CarpoolTripID, &p.PassengerName, &p.Origin, &p.Destination,
			&p.Seats, &p.AmountPaid, &p.Notes, &p.CreatedAt, &p.BoardStopIndex, &p.AlightStopIndex); err != nil {
			return err
		}
		t := &trips[index[p.CarpoolTripID]]
		t.Passengers = append(t.Passengers, p)
	}
	return pRows.Err()
}

func (r *Repository) DeleteCarpoolTrip(ctx context.Context, id, vehicleID string) error {
	query := `DELETE FROM carpool_trips WHERE id::text = $1 AND vehicle_id = $2;`
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
		s.CoverageRatePct = math.Round((s.TotalRevenue.Float()/s.TotalRealCost.Float())*1000) / 10
		s.TotalSaved = s.TotalRevenue
	}
	if s.TotalDistanceKm > 0 {
		s.NetCostPerKm = math.Round((s.TotalNetCost.Float()/s.TotalDistanceKm)*1000) / 1000
	}

	return &s, nil
}

func (r *Repository) GetDriveByID(ctx context.Context, driveID, vehicleID string) (*models.Drive, error) {
	query := `
		SELECT id, vehicle_id, teslamate_drive_id, start_time, end_time,
		       start_odometer, end_odometer, distance_km, duration_min,
		       speed_avg, speed_max, power_max, power_min, start_address, end_address, energy_consumed_kwh,
		       consumption_kwh_100km, tags, is_manual, toll_reviewed_at, created_at, updated_at
		FROM drives
		WHERE id::text = $1 AND vehicle_id = $2 AND deleted_upstream_at IS NULL;
	`
	var d models.Drive
	err := r.pool.QueryRow(ctx, query, driveID, vehicleID).Scan(
		&d.ID, &d.VehicleID, &d.TeslaMateDriveID, &d.StartTime, &d.EndTime,
		&d.StartOdometer, &d.EndOdometer, &d.DistanceKm, &d.DurationMin,
		&d.SpeedAvg, &d.SpeedMax, &d.PowerMax, &d.PowerMin,
		&d.StartAddress, &d.EndAddress, &d.EnergyConsumedKwh,
		&d.ConsumptionKwh100km, &d.Tags, &d.IsManual, &d.TollReviewedAt, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &d, nil
}

func (r *Repository) GetTripGroupDrives(ctx context.Context, vehicleID, tripGroupID string) ([]models.Drive, error) {
	query := `
		SELECT d.id, d.vehicle_id, d.teslamate_drive_id, d.start_time, d.end_time,
		       d.start_odometer, d.end_odometer, d.distance_km, d.duration_min,
		       d.speed_avg, d.speed_max, d.power_max, d.power_min, d.start_address, d.end_address, d.energy_consumed_kwh,
		       d.consumption_kwh_100km, d.tags, d.is_manual, d.toll_reviewed_at, d.created_at, d.updated_at
		FROM drives d
		JOIN trip_group_drives tgd ON d.id = tgd.drive_id
		JOIN trip_groups tg ON tg.id = tgd.trip_group_id
		WHERE tgd.trip_group_id::text = $1 AND tg.vehicle_id = $2 AND d.vehicle_id = $2 AND d.deleted_upstream_at IS NULL
		ORDER BY tgd.order_index ASC;
	`
	rows, err := r.pool.Query(ctx, query, tripGroupID, vehicleID)
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
			&d.SpeedAvg, &d.SpeedMax, &d.PowerMax, &d.PowerMin,
			&d.StartAddress, &d.EndAddress, &d.EnergyConsumedKwh,
			&d.ConsumptionKwh100km, &d.Tags, &d.IsManual, &d.TollReviewedAt, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, d)
	}
	return list, rows.Err()
}

// DriveTollAllocationCTE allocates drive expenses (in EUR) to drives of vehicle $1: an expense attached to
// a drive counts fully for it; an expense attached to a trip group is split across the group's drives
// proportionally to their distance (equally when the group has no distance).
const DriveTollAllocationCTE = `
	WITH group_stats AS (
		SELECT tgd.trip_group_id, SUM(d.distance_km) AS km, COUNT(*) AS n
		FROM trip_group_drives tgd
		JOIN trip_groups tg ON tg.id = tgd.trip_group_id AND tg.vehicle_id = $1
		JOIN drives d ON d.id = tgd.drive_id AND d.deleted_upstream_at IS NULL
		GROUP BY tgd.trip_group_id
	),
	allocations AS (
		SELECT e.id AS expense_id, e.drive_id, ` + AmountEURExpr + ` AS allocated
		FROM drive_expenses e
		JOIN drives d ON d.id = e.drive_id AND d.deleted_upstream_at IS NULL
		WHERE e.vehicle_id = $1
		UNION ALL
		SELECT e.id, tgd.drive_id,
		       ` + AmountEURExpr + ` * CASE WHEN gs.km > 0 THEN d.distance_km / gs.km ELSE 1.0 / gs.n END
		FROM drive_expenses e
		JOIN group_stats gs ON gs.trip_group_id = e.trip_group_id
		JOIN trip_group_drives tgd ON tgd.trip_group_id = e.trip_group_id
		JOIN drives d ON d.id = tgd.drive_id AND d.deleted_upstream_at IS NULL
		WHERE e.vehicle_id = $1
	)
`

// GetTollExpensesForDrives returns the drive expenses allocated to each drive (EUR).
func (r *Repository) GetTollExpensesForDrives(ctx context.Context, vehicleID string, driveIDs []string) (map[string]money.Cents, error) {
	result := make(map[string]money.Cents)
	ids := uniqueStrings(driveIDs)
	if len(ids) == 0 {
		return result, nil
	}

	rows, err := r.pool.Query(ctx, DriveTollAllocationCTE+`
		SELECT drive_id::text, COALESCE(SUM(allocated), 0)
		FROM allocations
		WHERE drive_id::text = ANY($2::text[])
		GROUP BY drive_id;
	`, vehicleID, ids)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var dID string
		var amt money.Cents
		if err := rows.Scan(&dID, &amt); err != nil {
			return result, err
		}
		result[dID] = amt
	}
	return result, rows.Err()
}

// GetTotalTollExpensesForDrives sums the expenses allocated to a set of drives (EUR).
func (r *Repository) GetTotalTollExpensesForDrives(ctx context.Context, vehicleID string, driveIDs []string) (money.Cents, error) {
	perDrive, err := r.GetTollExpensesForDrives(ctx, vehicleID, driveIDs)
	var total money.Cents
	for _, amt := range perDrive {
		total += amt
	}
	return total, err
}

// GetTollExpensesForTripGroup sums the expenses allocated to the drives of a trip group (EUR).
func (r *Repository) GetTollExpensesForTripGroup(ctx context.Context, vehicleID, tripGroupID string) (money.Cents, error) {
	drives, err := r.GetTripGroupDrives(ctx, vehicleID, tripGroupID)
	if err != nil {
		return 0, err
	}
	ids := make([]string, len(drives))
	for i, d := range drives {
		ids[i] = d.ID
	}
	return r.GetTotalTollExpensesForDrives(ctx, vehicleID, ids)
}

// GetDriveExpensesByDriveID lists expenses attached to a drive directly or through its trip groups,
// with the share allocated to this drive.
func (r *Repository) GetDriveExpensesByDriveID(ctx context.Context, vehicleID, driveID string) ([]models.DriveExpense, error) {
	rows, err := r.pool.Query(ctx, DriveTollAllocationCTE+`
		SELECT
			e.id, e.vehicle_id, e.trip_group_id, tg.name,
			e.drive_id,
			CASE
				WHEN d.id IS NOT NULL THEN COALESCE(NULLIF(d.start_address, ''), 'Départ') || ' → ' || COALESCE(NULLIF(d.end_address, ''), 'Arrivée')
				ELSE NULL
			END,
			e.type, e.amount, e.currency, e.fx_rate, e.date, e.notes,
			e.document_id, doc.filename,
			e.created_at,
			a.allocated
		FROM allocations a
		JOIN drive_expenses e ON e.id = a.expense_id
		LEFT JOIN drives d ON e.drive_id = d.id
		LEFT JOIN trip_groups tg ON e.trip_group_id = tg.id
		LEFT JOIN expense_documents doc ON e.document_id = doc.id
		WHERE a.drive_id::text = $2
		ORDER BY e.date ASC;
	`, vehicleID, driveID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.DriveExpense
	for rows.Next() {
		var de models.DriveExpense
		if err := rows.Scan(
			&de.ID, &de.VehicleID, &de.TripGroupID, &de.TripGroupName,
			&de.DriveID, &de.DriveTitle, &de.Type,
			&de.Amount, &de.Currency, &de.FxRate, &de.Date, &de.Notes,
			&de.DocumentID, &de.DocumentFilename,
			&de.CreatedAt,
			&de.AllocatedAmount,
		); err != nil {
			return nil, err
		}
		list = append(list, de)
	}
	return list, rows.Err()
}

// OdometerRange represents a half-open interval [Min, Max) of vehicle odometer readings.
// Max == nil means the session is still active (no upper bound).
type OdometerRange struct {
	Min float64
	Max *float64
}

// GetDrivingTelemetryStats returns driving dynamics averaged over the provided odometer ranges.
// Each range corresponds to a tire mount session: only drives whose end_odometer falls within
// [range.Min, range.Max) (or >= range.Min when Max is nil) are included.
// Returns zeros and count=0 when ranges is empty or no matching drives exist.
func (r *Repository) GetDrivingTelemetryStats(ctx context.Context, vehicleID string, ranges []OdometerRange) (avgPowerMax, avgPowerMin, avgConsumption float64, count int, err error) {
	if len(ranges) == 0 {
		return 0, 0, 0, 0, nil
	}

	// Build a WHERE clause with one OR-clause per range.
	args := []any{vehicleID}
	var clauses []string
	for _, rng := range ranges {
		lo := len(args) + 1
		hi := len(args) + 2
		args = append(args, rng.Min)
		if rng.Max != nil {
			args = append(args, *rng.Max)
			clauses = append(clauses, fmt.Sprintf("(end_odometer >= $%d AND end_odometer <= $%d)", lo, hi))
		} else {
			// Active session — upper bound is the vehicle's current odometer (no constraint needed)
			args = args[:len(args)-1] // drop the unused append
			clauses = append(clauses, fmt.Sprintf("(end_odometer >= $%d)", lo))
		}
	}

	query := fmt.Sprintf(`
		SELECT
			COALESCE(AVG(NULLIF(power_max, 0)), 0),
			COALESCE(AVG(NULLIF(power_min, 0)), 0),
			COALESCE(AVG(NULLIF(consumption_kwh_100km, 0)), 0),
			COUNT(*)
		FROM drives
		WHERE vehicle_id = $1 AND deleted_upstream_at IS NULL
		  AND (%s)
	`, strings.Join(clauses, " OR "))

	err = r.pool.QueryRow(ctx, query, args...).Scan(&avgPowerMax, &avgPowerMin, &avgConsumption, &count)
	if err != nil {
		return 0, 0, 0, 0, err
	}

	return avgPowerMax, avgPowerMin, avgConsumption, count, nil
}


// ============================================================================
// TeslaMate Reconciliation
// ============================================================================

// ReconcileResult reports records no longer present in TeslaMate.
type ReconcileResult struct {
	Missing int  // TeslaMate records of the covered window absent from the API response
	Marked  int  // Records flagged as deleted upstream
	Skipped bool // Guard triggered: too many records would disappear at once
}

// Share of the covered window above which missing records are considered an API anomaly rather than deletions.
const maxUpstreamDeletionShare = 0.2

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

// GetIdempotentResponse returns the stored response of a key, or nil.
func (r *Repository) GetIdempotentResponse(ctx context.Context, userID, key string) (*StoredResponse, error) {
	var res StoredResponse
	err := r.pool.QueryRow(ctx, `
		SELECT method, path, status_code, response_body FROM idempotency_keys WHERE user_id = $1 AND key = $2;
	`, userID, key).Scan(&res.Method, &res.Path, &res.StatusCode, &res.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &res, nil
}

// SaveIdempotentResponse stores the response of a key (first writer wins) and purges keys older than 30 days.
func (r *Repository) SaveIdempotentResponse(ctx context.Context, userID, key string, res StoredResponse) error {
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO idempotency_keys (user_id, key, method, path, status_code, response_body)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id, key) DO NOTHING;
	`, userID, key, res.Method, res.Path, res.StatusCode, res.Body); err != nil {
		return err
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE created_at < NOW() - INTERVAL '30 days';`)
	return err
}

// ============================================================================
// Vehicle Ownership
// ============================================================================

const ownershipColumns = `
	vehicle_id, acquisition_type, start_date, start_odometer,
	purchase_price, purchase_fees, incentives, expected_resale_value, expected_holding_months,
	loan_amount, loan_rate_pct, loan_duration_months, loan_fees, loan_insurance_monthly,
	lease_down_payment, lease_monthly_rent, lease_duration_months, lease_fees, lease_deposit,
	lease_km_allowance_per_year, lease_excess_km_price, lease_end_fees_estimate, lease_purchase_option_price,
	lease_includes_maintenance, lease_includes_insurance, lease_includes_tires, option_exercised_date,
	end_date, sale_price, created_at, updated_at
`

func scanOwnership(row pgx.Row, o *models.VehicleOwnership) error {
	return row.Scan(
		&o.VehicleID, &o.AcquisitionType, &o.StartDate, &o.StartOdometer,
		&o.PurchasePrice, &o.PurchaseFees, &o.Incentives, &o.ExpectedResaleValue, &o.ExpectedHoldingMonths,
		&o.LoanAmount, &o.LoanRatePct, &o.LoanDurationMonths, &o.LoanFees, &o.LoanInsuranceMonthly,
		&o.LeaseDownPayment, &o.LeaseMonthlyRent, &o.LeaseDurationMonths, &o.LeaseFees, &o.LeaseDeposit,
		&o.LeaseKmAllowancePerYear, &o.LeaseExcessKmPrice, &o.LeaseEndFeesEstimate, &o.LeasePurchaseOptionPrice,
		&o.LeaseIncludesMaintenance, &o.LeaseIncludesInsurance, &o.LeaseIncludesTires, &o.OptionExercisedDate,
		&o.EndDate, &o.SalePrice, &o.CreatedAt, &o.UpdatedAt,
	)
}

// GetVehicleOwnership returns the ownership contract of a vehicle, or ErrNotFound.
func (r *Repository) GetVehicleOwnership(ctx context.Context, vehicleID string) (*models.VehicleOwnership, error) {
	var o models.VehicleOwnership
	err := scanOwnership(r.pool.QueryRow(ctx, `SELECT `+ownershipColumns+` FROM vehicle_ownership WHERE vehicle_id = $1;`, vehicleID), &o)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &o, nil
}

// SaveVehicleOwnership creates or replaces the ownership contract of a vehicle.
func (r *Repository) SaveVehicleOwnership(ctx context.Context, o *models.VehicleOwnership) error {
	return scanOwnership(r.pool.QueryRow(ctx, `
		INSERT INTO vehicle_ownership (
			vehicle_id, acquisition_type, start_date, start_odometer,
			purchase_price, purchase_fees, incentives, expected_resale_value, expected_holding_months,
			loan_amount, loan_rate_pct, loan_duration_months, loan_fees, loan_insurance_monthly,
			lease_down_payment, lease_monthly_rent, lease_duration_months, lease_fees, lease_deposit,
			lease_km_allowance_per_year, lease_excess_km_price, lease_end_fees_estimate, lease_purchase_option_price,
			lease_includes_maintenance, lease_includes_insurance, lease_includes_tires, option_exercised_date,
			end_date, sale_price
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
		          $20, $21, $22, $23, $24, $25, $26, $27, $28, $29)
		ON CONFLICT (vehicle_id) DO UPDATE SET
			acquisition_type = EXCLUDED.acquisition_type, start_date = EXCLUDED.start_date,
			start_odometer = EXCLUDED.start_odometer, purchase_price = EXCLUDED.purchase_price,
			purchase_fees = EXCLUDED.purchase_fees, incentives = EXCLUDED.incentives,
			expected_resale_value = EXCLUDED.expected_resale_value, expected_holding_months = EXCLUDED.expected_holding_months,
			loan_amount = EXCLUDED.loan_amount, loan_rate_pct = EXCLUDED.loan_rate_pct,
			loan_duration_months = EXCLUDED.loan_duration_months, loan_fees = EXCLUDED.loan_fees,
			loan_insurance_monthly = EXCLUDED.loan_insurance_monthly, lease_down_payment = EXCLUDED.lease_down_payment,
			lease_monthly_rent = EXCLUDED.lease_monthly_rent, lease_duration_months = EXCLUDED.lease_duration_months,
			lease_fees = EXCLUDED.lease_fees, lease_deposit = EXCLUDED.lease_deposit,
			lease_km_allowance_per_year = EXCLUDED.lease_km_allowance_per_year,
			lease_excess_km_price = EXCLUDED.lease_excess_km_price, lease_end_fees_estimate = EXCLUDED.lease_end_fees_estimate,
			lease_purchase_option_price = EXCLUDED.lease_purchase_option_price,
			lease_includes_maintenance = EXCLUDED.lease_includes_maintenance,
			lease_includes_insurance = EXCLUDED.lease_includes_insurance, lease_includes_tires = EXCLUDED.lease_includes_tires,
			option_exercised_date = EXCLUDED.option_exercised_date, end_date = EXCLUDED.end_date,
			sale_price = EXCLUDED.sale_price, updated_at = NOW()
		RETURNING `+ownershipColumns+`;
	`,
		o.VehicleID, o.AcquisitionType, o.StartDate, o.StartOdometer,
		o.PurchasePrice, o.PurchaseFees, o.Incentives, o.ExpectedResaleValue, o.ExpectedHoldingMonths,
		o.LoanAmount, o.LoanRatePct, o.LoanDurationMonths, o.LoanFees, o.LoanInsuranceMonthly,
		o.LeaseDownPayment, o.LeaseMonthlyRent, o.LeaseDurationMonths, o.LeaseFees, o.LeaseDeposit,
		o.LeaseKmAllowancePerYear, o.LeaseExcessKmPrice, o.LeaseEndFeesEstimate, o.LeasePurchaseOptionPrice,
		o.LeaseIncludesMaintenance, o.LeaseIncludesInsurance, o.LeaseIncludesTires, o.OptionExercisedDate,
		o.EndDate, o.SalePrice,
	), o)
}

// DeleteVehicleOwnership removes the ownership contract of a vehicle.
func (r *Repository) DeleteVehicleOwnership(ctx context.Context, vehicleID string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM vehicle_ownership WHERE vehicle_id = $1;`, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListOdometerCheckpoints lists all manual odometer checkpoints for a vehicle, ordered by date ASC, odometer ASC.
func (r *Repository) ListOdometerCheckpoints(ctx context.Context, vehicleID string) ([]models.OdometerCheckpoint, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, vehicle_id, date, odometer, notes, created_at, updated_at
		FROM odometer_checkpoints
		WHERE vehicle_id = $1
		ORDER BY date ASC, odometer ASC;
	`, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := []models.OdometerCheckpoint{}
	for rows.Next() {
		var c models.OdometerCheckpoint
		if err := rows.Scan(&c.ID, &c.VehicleID, &c.Date, &c.Odometer, &c.Notes, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, c)
	}
	return list, rows.Err()
}

// CreateOdometerCheckpoint records a new odometer checkpoint.
func (r *Repository) CreateOdometerCheckpoint(ctx context.Context, c *models.OdometerCheckpoint) error {
	return r.pool.QueryRow(ctx, `
		INSERT INTO odometer_checkpoints (vehicle_id, date, odometer, notes)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at;
	`, c.VehicleID, c.Date, c.Odometer, c.Notes).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
}

// UpdateOdometerCheckpoint updates an existing odometer checkpoint.
func (r *Repository) UpdateOdometerCheckpoint(ctx context.Context, c *models.OdometerCheckpoint) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE odometer_checkpoints
		SET date = $3, odometer = $4, notes = $5, updated_at = NOW()
		WHERE id = $1 AND vehicle_id = $2;
	`, c.ID, c.VehicleID, c.Date, c.Odometer, c.Notes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteOdometerCheckpoint deletes an odometer checkpoint.
func (r *Repository) DeleteOdometerCheckpoint(ctx context.Context, vehicleID, checkpointID string) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM odometer_checkpoints
		WHERE id = $1 AND vehicle_id = $2;
	`, checkpointID, vehicleID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ============================================================================
// Expense Documents & Invoices
// ============================================================================

// SaveExpenseDocument stores a new uploaded document record in PostgreSQL.
// The binary data is stored on the filesystem volume; only the storage_path is persisted here.
func (r *Repository) SaveExpenseDocument(ctx context.Context, doc *models.ExpenseDocument) error {
	query := `
		INSERT INTO expense_documents (
			user_id, vehicle_id, filename, mime_type, file_size, storage_path, description
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at;
	`
	return r.pool.QueryRow(ctx, query,
		doc.UserID, doc.VehicleID, doc.Filename, doc.MimeType, doc.FileSize, doc.StoragePath, doc.Description,
	).Scan(&doc.ID, &doc.CreatedAt, &doc.UpdatedAt)
}

// GetExpenseDocumentByID retrieves an expense document metadata and its storage path.
// Access is verified via vehicles or vehicle_members.
func (r *Repository) GetExpenseDocumentByID(ctx context.Context, id, vehicleID, userID string) (*models.ExpenseDocument, error) {
	query := `
		SELECT d.id, d.user_id, d.vehicle_id, d.filename, d.mime_type, d.file_size, d.storage_path, d.description, d.created_at, d.updated_at
		FROM expense_documents d
		JOIN vehicles v ON v.id = d.vehicle_id
		WHERE d.id::text = $1 AND d.vehicle_id::text = $2 AND (v.user_id::text = $3 OR EXISTS (
			SELECT 1 FROM vehicle_members vm WHERE vm.vehicle_id = v.id AND vm.user_id::text = $3
		));
	`
	var doc models.ExpenseDocument
	err := r.pool.QueryRow(ctx, query, id, vehicleID, userID).Scan(
		&doc.ID, &doc.UserID, &doc.VehicleID, &doc.Filename, &doc.MimeType, &doc.FileSize, &doc.StoragePath, &doc.Description, &doc.CreatedAt, &doc.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &doc, nil
}

// ListExpenseDocuments lists document headers for a vehicle, including how many expenses link to each document.
func (r *Repository) ListExpenseDocuments(ctx context.Context, vehicleID, userID string) ([]models.ExpenseDocumentHeader, error) {
	query := `
		SELECT
			d.id, d.vehicle_id, d.filename, d.mime_type, d.file_size, d.description,
			(
				(SELECT COUNT(*) FROM drive_expenses de WHERE de.document_id = d.id) +
				(SELECT COUNT(*) FROM maintenance_expenses me WHERE me.document_id = d.id) +
				(SELECT COUNT(*) FROM charge_logs cl WHERE cl.document_id = d.id)
			) AS linked_expenses_count,
			d.created_at
		FROM expense_documents d
		JOIN vehicles v ON v.id = d.vehicle_id
		WHERE d.vehicle_id::text = $1 AND (v.user_id::text = $2 OR EXISTS (
			SELECT 1 FROM vehicle_members vm WHERE vm.vehicle_id = v.id AND vm.user_id::text = $2
		))
		ORDER BY d.created_at DESC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.ExpenseDocumentHeader
	for rows.Next() {
		var h models.ExpenseDocumentHeader
		if err := rows.Scan(
			&h.ID, &h.VehicleID, &h.Filename, &h.MimeType, &h.FileSize, &h.Description,
			&h.LinkedExpensesCount, &h.CreatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, h)
	}
	return list, rows.Err()
}

// DeleteExpenseDocument deletes an expense document. Linked expenses have their document_id set to NULL automatically.
func (r *Repository) DeleteExpenseDocument(ctx context.Context, id, vehicleID, userID string) error {
	query := `
		DELETE FROM expense_documents d
		USING vehicles v
		WHERE d.vehicle_id = v.id AND d.id::text = $1 AND d.vehicle_id::text = $2 AND (v.user_id::text = $3 OR EXISTS (
			SELECT 1 FROM vehicle_members vm WHERE vm.vehicle_id = v.id AND vm.user_id::text = $3 AND vm.role IN ('OWNER', 'EDITOR')
		));
	`
	cmdTag, err := r.pool.Exec(ctx, query, id, vehicleID, userID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateDocumentStoragePath sets the storage_path for a document after the file has been written to the volume.
func (r *Repository) UpdateDocumentStoragePath(ctx context.Context, docID, storagePath string) error {
	query := `UPDATE expense_documents SET storage_path = $1 WHERE id::text = $2;`
	_, err := r.pool.Exec(ctx, query, storagePath, docID)
	return err
}

// ============================================================================
// Maintenance Reminders & Webhooks
// ============================================================================

func (r *Repository) ListMaintenanceReminders(ctx context.Context, vehicleID string, currentOdo float64) ([]models.MaintenanceReminder, error) {
	query := `
		SELECT id, vehicle_id, title, category, interval_km, interval_months,
		       last_service_odometer, last_service_date, lead_km, lead_days,
		       webhook_enabled, last_notified_at, last_notified_odometer,
		       created_at, updated_at
		FROM maintenance_reminders
		WHERE vehicle_id = $1
		ORDER BY created_at ASC;
	`
	rows, err := r.pool.Query(ctx, query, vehicleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	var list []models.MaintenanceReminder
	for rows.Next() {
		var rem models.MaintenanceReminder
		if err := rows.Scan(
			&rem.ID, &rem.VehicleID, &rem.Title, &rem.Category, &rem.IntervalKm, &rem.IntervalMonths,
			&rem.LastServiceOdometer, &rem.LastServiceDate, &rem.LeadKm, &rem.LeadDays,
			&rem.WebhookEnabled, &rem.LastNotifiedAt, &rem.LastNotifiedOdometer,
			&rem.CreatedAt, &rem.UpdatedAt,
		); err != nil {
			return nil, err
		}
		rem.ComputeStatus(currentOdo, now)
		list = append(list, rem)
	}

	// Sort by urgency: OVERDUE first, then DUE_SOON, then OK
	sort.Slice(list, func(i, j int) bool {
		priority := func(s string) int {
			switch s {
			case "OVERDUE":
				return 0
			case "DUE_SOON":
				return 1
			default:
				return 2
			}
		}
		pI := priority(list[i].Status)
		pJ := priority(list[j].Status)
		if pI != pJ {
			return pI < pJ
		}
		remI := 99999999.0
		if list[i].RemainingKm != nil {
			remI = *list[i].RemainingKm
		}
		remJ := 99999999.0
		if list[j].RemainingKm != nil {
			remJ = *list[j].RemainingKm
		}
		return remI < remJ
	})

	return list, rows.Err()
}

func (r *Repository) GetMaintenanceReminderByID(ctx context.Context, vehicleID, reminderID string, currentOdo float64) (*models.MaintenanceReminder, error) {
	query := `
		SELECT id, vehicle_id, title, category, interval_km, interval_months,
		       last_service_odometer, last_service_date, lead_km, lead_days,
		       webhook_enabled, last_notified_at, last_notified_odometer,
		       created_at, updated_at
		FROM maintenance_reminders
		WHERE vehicle_id = $1 AND id::text = $2;
	`
	var rem models.MaintenanceReminder
	err := r.pool.QueryRow(ctx, query, vehicleID, reminderID).Scan(
		&rem.ID, &rem.VehicleID, &rem.Title, &rem.Category, &rem.IntervalKm, &rem.IntervalMonths,
		&rem.LastServiceOdometer, &rem.LastServiceDate, &rem.LeadKm, &rem.LeadDays,
		&rem.WebhookEnabled, &rem.LastNotifiedAt, &rem.LastNotifiedOdometer,
		&rem.CreatedAt, &rem.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	rem.ComputeStatus(currentOdo, time.Now())
	return &rem, nil
}

func (r *Repository) CreateMaintenanceReminder(ctx context.Context, rem *models.MaintenanceReminder) error {
	query := `
		INSERT INTO maintenance_reminders (
			vehicle_id, title, category, interval_km, interval_months,
			last_service_odometer, last_service_date, lead_km, lead_days,
			webhook_enabled
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at;
	`
	return r.pool.QueryRow(ctx, query,
		rem.VehicleID, rem.Title, rem.Category, rem.IntervalKm, rem.IntervalMonths,
		rem.LastServiceOdometer, rem.LastServiceDate, rem.LeadKm, rem.LeadDays,
		rem.WebhookEnabled,
	).Scan(&rem.ID, &rem.CreatedAt, &rem.UpdatedAt)
}

func (r *Repository) UpdateMaintenanceReminder(ctx context.Context, rem *models.MaintenanceReminder) error {
	query := `
		UPDATE maintenance_reminders
		SET title = $1, category = $2, interval_km = $3, interval_months = $4,
		    last_service_odometer = $5, last_service_date = $6, lead_km = $7, lead_days = $8,
		    webhook_enabled = $9, updated_at = NOW()
		WHERE id::text = $10 AND vehicle_id = $11;
	`
	cmdTag, err := r.pool.Exec(ctx, query,
		rem.Title, rem.Category, rem.IntervalKm, rem.IntervalMonths,
		rem.LastServiceOdometer, rem.LastServiceDate, rem.LeadKm, rem.LeadDays,
		rem.WebhookEnabled, rem.ID, rem.VehicleID,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) CompleteMaintenanceReminder(ctx context.Context, vehicleID, reminderID string, completedDate time.Time, completedOdo float64) error {
	query := `
		UPDATE maintenance_reminders
		SET last_service_date = $1, last_service_odometer = $2, last_notified_at = NULL, last_notified_odometer = NULL, updated_at = NOW()
		WHERE id::text = $3 AND vehicle_id = $4;
	`
	cmdTag, err := r.pool.Exec(ctx, query, completedDate, completedOdo, reminderID, vehicleID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteMaintenanceReminder(ctx context.Context, vehicleID, reminderID string) error {
	query := `DELETE FROM maintenance_reminders WHERE id::text = $1 AND vehicle_id = $2;`
	cmdTag, err := r.pool.Exec(ctx, query, reminderID, vehicleID)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) MarkReminderNotified(ctx context.Context, reminderID string, notifiedAt time.Time, notifiedOdo float64) error {
	query := `
		UPDATE maintenance_reminders
		SET last_notified_at = $1, last_notified_odometer = $2
		WHERE id::text = $3;
	`
	_, err := r.pool.Exec(ctx, query, notifiedAt, notifiedOdo, reminderID)
	return err
}

func (r *Repository) GetVehicleWebhook(ctx context.Context, vehicleID string) (*models.VehicleWebhook, error) {
	query := `
		SELECT id, vehicle_id, url, type, enabled, created_at, updated_at
		FROM vehicle_webhooks
		WHERE vehicle_id = $1;
	`
	var w models.VehicleWebhook
	err := r.pool.QueryRow(ctx, query, vehicleID).Scan(
		&w.ID, &w.VehicleID, &w.URL, &w.Type, &w.Enabled, &w.CreatedAt, &w.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No webhook configured yet
		}
		return nil, err
	}
	return &w, nil
}

func (r *Repository) UpsertVehicleWebhook(ctx context.Context, w *models.VehicleWebhook) error {
	query := `
		INSERT INTO vehicle_webhooks (vehicle_id, url, type, enabled)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (vehicle_id) DO UPDATE
		SET url = EXCLUDED.url, type = EXCLUDED.type, enabled = EXCLUDED.enabled, updated_at = NOW()
		RETURNING id, created_at, updated_at;
	`
	return r.pool.QueryRow(ctx, query, w.VehicleID, w.URL, w.Type, w.Enabled).Scan(&w.ID, &w.CreatedAt, &w.UpdatedAt)
}

func (r *Repository) DeleteVehicleWebhook(ctx context.Context, vehicleID string) error {
	query := `DELETE FROM vehicle_webhooks WHERE vehicle_id = $1;`
	_, err := r.pool.Exec(ctx, query, vehicleID)
	return err
}

