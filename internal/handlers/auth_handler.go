package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/config"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
)

const (
	oidcStateCookie     = "oidc_state"
	oidcNonceCookie     = "oidc_nonce"
	oidcCookieTTL       = 10 * time.Minute
	refreshTokenCookie  = "teslacost_refresh_token"
)

type AuthHandler struct {
	repo        *database.Repository
	cfg         *config.Config
	oidcService *auth.OIDCService // nil when OIDC is not configured
}

func NewAuthHandler(repo *database.Repository, cfg *config.Config, oidcService *auth.OIDCService) *AuthHandler {
	return &AuthHandler{repo: repo, cfg: cfg, oidcService: oidcService}
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthResponse struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	User         any    `json:"user"`
}

func (h *AuthHandler) setRefreshTokenCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookie,
		Value:    token,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

func (h *AuthHandler) clearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookie,
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

func (h *AuthHandler) issueSession(w http.ResponseWriter, r *http.Request, userID, email, familyID string) (string, string, error) {
	accessToken, err := auth.GenerateAccessToken(userID, email, h.cfg.JWTSecret, h.cfg.JWTAccessExpirationMinutes)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate access token: %w", err)
	}

	plainRefreshToken, tokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	if familyID == "" {
		newFamID, famErr := auth.NewUUID()
		if famErr != nil {
			return "", "", fmt.Errorf("failed to generate family ID: %w", famErr)
		}
		familyID = newFamID
	}

	expiresAt := time.Now().Add(time.Duration(h.cfg.JWTRefreshExpirationDays) * 24 * time.Hour)
	ip := r.RemoteAddr
	ua := r.UserAgent()
	var ipPtr, uaPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	if ua != "" {
		uaPtr = &ua
	}

	_, err = h.repo.CreateRefreshToken(r.Context(), userID, tokenHash, familyID, expiresAt, ipPtr, uaPtr)
	if err != nil {
		return "", "", fmt.Errorf("failed to persist refresh token: %w", err)
	}

	h.setRefreshTokenCookie(w, plainRefreshToken, expiresAt)
	return accessToken, plainRefreshToken, nil
}

// GetConfig returns public authentication configuration used by the frontend.
func (h *AuthHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	count, err := h.repo.GetUserCount(r.Context())
	userCount := 0
	if err == nil {
		userCount = count
	}
	needsOnboarding := (userCount == 0)

	regEnabled := !h.cfg.DisableRegistration
	if needsOnboarding {
		regEnabled = true
	}
	if h.cfg.OIDCEnabled && h.cfg.OIDCDisableLocalAuth {
		regEnabled = false
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"registration_enabled": regEnabled,
		"needs_onboarding":     needsOnboarding,
		"user_count":           userCount,
		"oidc_enabled":         h.cfg.OIDCEnabled,
		"oidc_provider_name":   h.cfg.OIDCProviderName,
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if h.cfg.OIDCEnabled && h.cfg.OIDCDisableLocalAuth {
		writeError(w, http.StatusForbidden, "L'inscription locale est desactivee - utilisez le SSO")
		return
	}
	if h.cfg.DisableRegistration {
		count, err := h.repo.GetUserCount(r.Context())
		if err != nil || count > 0 {
			writeError(w, http.StatusForbidden, "La creation de compte est desactivee sur cette instance")
			return
		}
	}

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.Email == "" || len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "Email required and password must be at least 8 characters")
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}
	user, err := h.repo.CreateUser(r.Context(), req.Email, hash)
	if err != nil {
		writeError(w, http.StatusConflict, "Email already registered or creation failed")
		return
	}

	accessToken, refreshToken, err := h.issueSession(w, r, user.ID, user.Email, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate session tokens")
		return
	}
	writeJSON(w, http.StatusCreated, AuthResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if h.cfg.OIDCEnabled && h.cfg.OIDCDisableLocalAuth {
		writeError(w, http.StatusForbidden, "La connexion locale est desactivee - utilisez le SSO")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.repo.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}
	if user.PasswordHash == nil || !auth.CheckPassword(req.Password, *user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	accessToken, refreshToken, err := h.issueSession(w, r, user.ID, user.Email, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate session tokens")
		return
	}
	writeJSON(w, http.StatusOK, AuthResponse{
		Token:        accessToken,
		RefreshToken: refreshToken,
		User:         user,
	})
}

func (h *AuthHandler) RefreshToken(w http.ResponseWriter, r *http.Request) {
	var plainToken string

	// 1. Check HttpOnly cookie
	if cookie, err := r.Cookie(refreshTokenCookie); err == nil && cookie.Value != "" {
		plainToken = cookie.Value
	}

	// 2. Check JSON payload fallback
	if plainToken == "" && r.Body != nil {
		var req RefreshRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		plainToken = req.RefreshToken
	}

	if plainToken == "" {
		writeError(w, http.StatusUnauthorized, "Missing refresh token")
		return
	}

	oldTokenHash := auth.HashRefreshToken(plainToken)

	newPlainToken, newTokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate new refresh token")
		return
	}

	expiresAt := time.Now().Add(time.Duration(h.cfg.JWTRefreshExpirationDays) * 24 * time.Hour)
	ip := r.RemoteAddr
	ua := r.UserAgent()
	var ipPtr, uaPtr *string
	if ip != "" {
		ipPtr = &ip
	}
	if ua != "" {
		uaPtr = &ua
	}

	rotatedToken, err := h.repo.RotateRefreshToken(r.Context(), oldTokenHash, newTokenHash, expiresAt, ipPtr, uaPtr)
	if err != nil {
		h.clearRefreshTokenCookie(w)
		if errors.Is(err, database.ErrRefreshTokenReused) {
			writeError(w, http.StatusUnauthorized, "Security alert: refresh token reuse detected")
			return
		}
		if errors.Is(err, database.ErrNotFound) || err.Error() == "refresh token expired" {
			writeError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to refresh token")
		return
	}

	user, err := h.repo.GetUserByID(r.Context(), rotatedToken.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "User account no longer exists")
		return
	}

	accessToken, err := auth.GenerateAccessToken(user.ID, user.Email, h.cfg.JWTSecret, h.cfg.JWTAccessExpirationMinutes)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate access token")
		return
	}

	h.setRefreshTokenCookie(w, newPlainToken, expiresAt)
	writeJSON(w, http.StatusOK, AuthResponse{
		Token:        accessToken,
		RefreshToken: newPlainToken,
		User:         user,
	})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var plainToken string
	if cookie, err := r.Cookie(refreshTokenCookie); err == nil && cookie.Value != "" {
		plainToken = cookie.Value
	}
	if plainToken == "" && r.Body != nil {
		var req RefreshRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		plainToken = req.RefreshToken
	}

	if plainToken != "" && h.repo != nil {
		tokenHash := auth.HashRefreshToken(plainToken)
		_ = h.repo.RevokeRefreshToken(r.Context(), tokenHash)
	}

	h.clearRefreshTokenCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{
		"message": "Logged out successfully",
	})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	user, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "User not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// OIDCLogin initiates the Authorization Code Flow: generates state + nonce cookies,
// then redirects the browser to the IdP authorization endpoint.
func (h *AuthHandler) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	if h.oidcService == nil {
		writeError(w, http.StatusNotFound, "OIDC is not configured on this instance")
		return
	}

	state, err := auth.GenerateStateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate OIDC state")
		return
	}
	noncePlain, nonceHashed, err := auth.GenerateNonce()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate OIDC nonce")
		return
	}

	// Secure flag is set to true for OIDC state/nonce cookies.
	expire := time.Now().Add(oidcCookieTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		Expires:  expire,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oidcNonceCookie,
		Value:    noncePlain,
		Expires:  expire,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	http.Redirect(w, r, h.oidcService.LoginURL(state, nonceHashed), http.StatusFound)
}

// OIDCCallback handles the redirect from the IdP, exchanges the authorization code,
// performs JIT user provisioning, issues a TeslaCost JWT + Refresh Token, and redirects to the SPA.
func (h *AuthHandler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	if h.oidcService == nil {
		writeError(w, http.StatusNotFound, "OIDC is not configured on this instance")
		return
	}

	// Verify and consume the state cookie (one-time CSRF token).
	stateCookie, err := r.Cookie(oidcStateCookie)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Missing OIDC state cookie - session may have expired")
		return
	}
	clearCookie(w, oidcStateCookie)

	if r.URL.Query().Get("state") != stateCookie.Value {
		writeError(w, http.StatusBadRequest, "OIDC state mismatch - possible CSRF attempt")
		return
	}

	nonceCookie, err := r.Cookie(oidcNonceCookie)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Missing OIDC nonce cookie - session may have expired")
		return
	}
	clearCookie(w, oidcNonceCookie)

	code := r.URL.Query().Get("code")
	if code == "" {
		oidcErr := r.URL.Query().Get("error")
		writeError(w, http.StatusBadRequest, fmt.Sprintf("OIDC error: %s", oidcErr))
		return
	}

	userInfo, err := h.oidcService.ExchangeCode(r.Context(), code, nonceCookie.Value)
	if err != nil {
		if errors.Is(err, auth.ErrEmailNotAllowed) {
			writeError(w, http.StatusForbidden, "Your email address is not authorized on this instance")
			return
		}
		writeError(w, http.StatusUnauthorized, fmt.Sprintf("OIDC authentication failed: %v", err))
		return
	}

	// JIT provisioning: create or link the user account.
	dbUser, err := h.repo.UpsertOIDCUser(r.Context(),
		userInfo.Email, userInfo.Subject, h.cfg.OIDCIssuerURL, userInfo.DisplayName,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to provision user account")
		return
	}

	accessToken, _, err := h.issueSession(w, r, dbUser.ID, dbUser.Email, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	http.Redirect(w, r, "/oidc-callback?token="+url.QueryEscape(accessToken), http.StatusFound)
}

// clearCookie immediately expires a named cookie.
func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}