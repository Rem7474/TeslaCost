package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/teslacost/teslacost/internal/models"
)

// Users, password auth and refresh-token session management.

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
