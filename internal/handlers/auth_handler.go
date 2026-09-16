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

// oidcStateCookie / oidcNonceCookie are the HttpOnly cookies used for CSRF + nonce protection.
const (
	oidcStateCookie = "oidc_state"
	oidcNonceCookie = "oidc_nonce"
	oidcCookieTTL   = 10 * time.Minute
)

type AuthHandler struct {
	repo                *database.Repository
	cfg                 *config.Config
	oidcService         *auth.OIDCService // nil when OIDC is not configured
}

func NewAuthHandler(
	repo *database.Repository,
	cfg *config.Config,
	oidcService *auth.OIDCService,
) *AuthHandler {
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

type AuthResponse struct {
	Token string `json:"token"`
	User  any    `json:"user"`
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
	// Disable local registration when OIDC is active + local auth is disabled.
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
	// Block local registration when OIDC + disable-local-auth is configured.
	if h.cfg.OIDCEnabled && h.cfg.OIDCDisableLocalAuth {
		writeError(w, http.StatusForbidden, "L'inscription locale est désactivée — utilisez le SSO")
		return
	}

	if h.cfg.DisableRegistration {
		count, err := h.repo.GetUserCount(r.Context())
		if err != nil || count > 0 {
			writeError(w, http.StatusForbidden, "La création de compte est désactivée sur cette instance")
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

	token, err := auth.GenerateToken(user.ID, user.Email, h.cfg.JWTSecret, h.cfg.JWTExpirationHours)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, AuthResponse{Token: token, User: user})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Block local login when OIDC + disable-local-auth is configured.
	if h.cfg.OIDCEnabled && h.cfg.OIDCDisableLocalAuth {
		writeError(w, http.StatusForbidden, "La connexion locale est désactivée — utilisez le SSO")
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
	// OIDC-only accounts have no local password.
	if user.PasswordHash == nil || !auth.CheckPassword(req.Password, *user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Email, h.cfg.JWTSecret, h.cfg.JWTExpirationHours)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{Token: token, User: user})
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

// ============================================================================
// OIDC / SSO endpoints
// ============================================================================

// OIDCLogin initiates the Authorization Code Flow: generates state + nonce,
// stores them in HttpOnly cookies, and redirects the browser to the IdP.
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

	// Store state and nonce in HttpOnly cookies (SameSite=Lax, TTL 10 min).
	expire := time.Now().Add(oidcCookieTTL)
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		Expires:  expire,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oidcNonceCookie,
		Value:    noncePlain,
		Expires:  expire,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})

	http.Redirect(w, r, h.oidcService.LoginURL(state, nonceHashed), http.StatusFound)
}

// OIDCCallback handles the redirect from the IdP after successful authentication.
// It verifies the state, exchanges the code, upserts the user, issues a JWT, and
// redirects the browser to the SPA callback page with the token as a query param.
func (h *AuthHandler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	if h.oidcService == nil {
		writeError(w, http.StatusNotFound, "OIDC is not configured on this instance")
		return
	}

	// Read and immediately clear the state cookie (one-time use).
	stateCookie, err := r.Cookie(oidcStateCookie)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Missing OIDC state cookie — session may have expired")
		return
	}
	clearCookie(w, oidcStateCookie)

	if r.URL.Query().Get("state") != stateCookie.Value {
		writeError(w, http.StatusBadRequest, "OIDC state mismatch — possible CSRF attempt")
		return
	}

	// Read and clear the nonce cookie.
	nonceCookie, err := r.Cookie(oidcNonceCookie)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Missing OIDC nonce cookie — session may have expired")
		return
	}
	clearCookie(w, oidcNonceCookie)

	// Exchange authorization code for ID token.
	code := r.URL.Query().Get("code")
	if code == "" {
		// IdP may send an error parameter (e.g. access_denied).
		oidcErr := r.URL.Query().Get("error")
		writeError(w, http.StatusBadRequest, fmt.Sprintf("OIDC error: %s", oidcErr))
		return
	}

	userInfo, err := h.oidcService.ExchangeCode(r.Context(), code, nonceCookie.Value)
	if err != nil {
		if errors.Is(err, auth.ErrEmailNotAllowed) {
			writeError(w, http.StatusForbidden, "Votre adresse email n'est pas autorisée sur cette instance")
			return
		}
		writeError(w, http.StatusUnauthorized, fmt.Sprintf("OIDC authentication failed: %v", err))
		return
	}

	// JIT provisioning: upsert user in our DB.
	dbUser, err := h.repo.UpsertOIDCUser(r.Context(),
		userInfo.Email,
		userInfo.Subject,
		h.cfg.OIDCIssuerURL, // use issuer URL as the stable "provider" identifier
		userInfo.DisplayName,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to provision user account")
		return
	}

	// Issue our standard JWT — identical format to local auth.
	token, err := auth.GenerateToken(dbUser.ID, dbUser.Email, h.cfg.JWTSecret, h.cfg.JWTExpirationHours)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// Redirect the browser to the SPA callback handler with the token.
	redirectURL := "/oidc-callback?token=" + url.QueryEscape(token)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// clearCookie deletes a cookie by setting it to expired.
func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}