package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/config"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
)

const (
	oidcStateCookie    = "oidc_state"
	oidcNonceCookie    = "oidc_nonce"
	oidcVerifierCookie = "oidc_verifier"
	oidcCookieTTL      = 10 * time.Minute
	refreshTokenCookie = "teslacost_refresh_token"
	accessTokenCookie  = "teslacost_access_token"
)

type AuthHandler struct {
	repo        *database.Repository
	cfg         *config.Config
	oidcService *auth.OIDCService // nil when OIDC is not configured
	throttle    *auth.LoginThrottle
}

func NewAuthHandler(repo *database.Repository, cfg *config.Config, oidcService *auth.OIDCService) *AuthHandler {
	return &AuthHandler{repo: repo, cfg: cfg, oidcService: oidcService, throttle: auth.NewLoginThrottle()}
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

// AuthResponse carries the short-lived access token and the user. The refresh token is only ever sent as an
// HttpOnly cookie: put in the body it would be readable by any script running in the page.
type AuthResponse struct {
	Token string `json:"token"`
	User  any    `json:"user"`
}

// Secure is deliberately config-driven (cfg.CookieSecure: true in production, false
// otherwise) rather than a hardcoded literal true on every cookie below. Forcing Secure
// unconditionally would silently break every cookie-based session over plain HTTP on any
// host other than literal "localhost" (the one origin browsers special-case as trustworthy
// without TLS) — that was a real pre-existing bug this fixes. Static analysis can't verify a
// non-literal boolean is "secure enough" and flags it regardless; NOSONAR below is intentional.

func (h *AuthHandler) setRefreshTokenCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{ // NOSONAR
		Name:     refreshTokenCookie,
		Value:    token,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

func (h *AuthHandler) clearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{ // NOSONAR
		Name:     refreshTokenCookie,
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

// setAccessTokenCookie stores the short-lived JWT access token in an HttpOnly cookie so the
// frontend never needs to hold it in JS-readable storage (localStorage), removing it as an
// XSS exfiltration target. The token is also still returned in the JSON body for non-browser
// API consumers that prefer a Bearer header.
func (h *AuthHandler) setAccessTokenCookie(w http.ResponseWriter, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{ // NOSONAR
		Name:     accessTokenCookie,
		Value:    token,
		Expires:  expiresAt,
		MaxAge:   int(time.Until(expiresAt).Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

func (h *AuthHandler) clearAccessTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{ // NOSONAR
		Name:     accessTokenCookie,
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

// clearCookie immediately expires a named cookie (used for the short-lived OIDC state/nonce cookies).
func (h *AuthHandler) clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{ // NOSONAR
		Name:     name,
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
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
	accessTokenExpiresAt := time.Now().Add(time.Duration(h.cfg.JWTAccessExpirationMinutes) * time.Minute)
	h.setAccessTokenCookie(w, accessToken, accessTokenExpiresAt)
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
	if len(req.Password) > auth.MaxPasswordBytes {
		writeError(w, http.StatusBadRequest, "Le mot de passe ne doit pas dépasser 72 octets")
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

	accessToken, _, err := h.issueSession(w, r, user.ID, user.Email, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate session tokens")
		return
	}
	writeJSON(w, http.StatusCreated, AuthResponse{Token: accessToken, User: user})
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

	if blocked, retryAfter := h.throttle.Blocked(req.Email); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, "Trop de tentatives de connexion pour ce compte : réessayez plus tard")
		return
	}

	// An unknown address and a wrong password take the same time and give the same answer.
	user, err := h.repo.GetUserByEmail(r.Context(), req.Email)
	if err != nil || user.PasswordHash == nil {
		auth.CheckPasswordAgainstNobody(req.Password)
		h.throttle.Fail(req.Email)
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}
	if !auth.CheckPassword(req.Password, *user.PasswordHash) {
		h.throttle.Fail(req.Email)
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	h.throttle.Reset(req.Email)
	accessToken, _, err := h.issueSession(w, r, user.ID, user.Email, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate session tokens")
		return
	}
	writeJSON(w, http.StatusOK, AuthResponse{Token: accessToken, User: user})
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
	accessTokenExpiresAt := time.Now().Add(time.Duration(h.cfg.JWTAccessExpirationMinutes) * time.Minute)
	h.setAccessTokenCookie(w, accessToken, accessTokenExpiresAt)
	writeJSON(w, http.StatusOK, AuthResponse{Token: accessToken, User: user})
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
	h.clearAccessTokenCookie(w)
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

	verifier := auth.GenerateCodeVerifier()

	expire := time.Now().Add(oidcCookieTTL)
	http.SetCookie(w, &http.Cookie{ // NOSONAR - Secure is config-driven (cfg.CookieSecure), see comment on setRefreshTokenCookie.
		Name:     oidcStateCookie,
		Value:    state,
		Expires:  expire,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	http.SetCookie(w, &http.Cookie{ // NOSONAR - Secure is config-driven (cfg.CookieSecure), see comment on setRefreshTokenCookie.
		Name:     oidcNonceCookie,
		Value:    noncePlain,
		Expires:  expire,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	http.SetCookie(w, &http.Cookie{ // NOSONAR - Secure is config-driven (cfg.CookieSecure), see comment on setRefreshTokenCookie.
		Name:     oidcVerifierCookie,
		Value:    verifier,
		Expires:  expire,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	http.Redirect(w, r, h.oidcService.LoginURL(state, nonceHashed, verifier), http.StatusFound)
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
	h.clearCookie(w, oidcStateCookie)

	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("state")), []byte(stateCookie.Value)) != 1 {
		writeError(w, http.StatusBadRequest, "OIDC state mismatch - possible CSRF attempt")
		return
	}

	nonceCookie, err := r.Cookie(oidcNonceCookie)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Missing OIDC nonce cookie - session may have expired")
		return
	}
	h.clearCookie(w, oidcNonceCookie)

	verifierCookie, err := r.Cookie(oidcVerifierCookie)
	if err != nil || verifierCookie.Value == "" {
		writeError(w, http.StatusBadRequest, "Missing OIDC verifier cookie - session may have expired")
		return
	}
	h.clearCookie(w, oidcVerifierCookie)

	code := r.URL.Query().Get("code")
	if code == "" {
		oidcErr := r.URL.Query().Get("error")
		writeError(w, http.StatusBadRequest, fmt.Sprintf("OIDC error: %s", oidcErr))
		return
	}

	userInfo, err := h.oidcService.ExchangeCode(r.Context(), code, nonceCookie.Value, verifierCookie.Value)
	if err != nil {
		if errors.Is(err, auth.ErrEmailNotAllowed) {
			writeError(w, http.StatusForbidden, "Your email address is not authorized on this instance")
			return
		}
		if errors.Is(err, auth.ErrEmailNotVerified) {
			writeError(w, http.StatusForbidden, "Votre fournisseur d'identité n'a pas vérifié votre adresse e-mail")
			return
		}
		// The detail (token endpoint answers, claim names) is for the logs, not for whoever sent the request.
		slog.WarnContext(r.Context(), "OIDC authentication failed", "component", "auth", "error", err)
		writeError(w, http.StatusUnauthorized, "OIDC authentication failed")
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

	_, _, err = h.issueSession(w, r, dbUser.ID, dbUser.Email, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Both the refresh and access token cookies were already set on this response by
	// issueSession; the SPA just needs to know the handshake succeeded.
	http.Redirect(w, r, "/oidc-callback", http.StatusFound)
}
