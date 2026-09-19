package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
)

// currentFamily returns the session (refresh token family) of the request, or "" for a client that does not send
// the refresh cookie (a script using a bearer token).
func (h *AuthHandler) currentFamily(r *http.Request) string {
	cookie, err := r.Cookie(refreshTokenCookie)
	if err != nil || cookie.Value == "" {
		return ""
	}
	token, err := h.repo.GetRefreshTokenByHash(r.Context(), auth.HashRefreshToken(cookie.Value))
	if err != nil {
		return ""
	}
	// A token of someone else's session does not make it this user's current one
	if token.UserID != middleware.GetUserID(r.Context()) {
		return ""
	}
	return token.FamilyID
}

// ListSessions lists the devices signed in to the account, the current one flagged.
func (h *AuthHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	sessions, err := h.repo.ListSessions(r.Context(), userID)
	if err != nil {
		writeRepoError(w, r, err, "Impossible de lister les sessions")
		return
	}
	current := h.currentFamily(r)
	for i := range sessions {
		sessions[i].Current = current != "" && sessions[i].ID == current
	}
	writeJSON(w, http.StatusOK, sessions)
}

// RevokeSession signs a device out. Closing the current session also clears its cookies.
func (h *AuthHandler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := chi.URLParam(r, "sessionId")
	current := h.currentFamily(r)

	found, err := h.repo.RevokeUserSession(r.Context(), userID, id)
	if err != nil {
		writeRepoError(w, r, err, "Impossible de révoquer la session")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, "Session introuvable")
		return
	}
	if id == current {
		h.clearRefreshTokenCookie(w)
		h.clearAccessTokenCookie(w)
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Session révoquée"})
}

// LogoutAll signs every device out, the current one included.
func (h *AuthHandler) LogoutAll(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	revoked, err := h.repo.RevokeUserSessionsExcept(r.Context(), userID, "")
	if err != nil {
		writeRepoError(w, r, err, "Impossible de révoquer les sessions")
		return
	}
	h.clearRefreshTokenCookie(w)
	h.clearAccessTokenCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"message": "Toutes les sessions sont révoquées", "sessions_revoked": revoked})
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// ChangePassword replaces the local password after checking the current one, and signs out every other device: the
// usual reason to change a password is that someone else may have it. Wrong answers are refused with 403, not 401,
// which the frontend would take for an expired session.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if h.cfg.OIDCEnabled && h.cfg.OIDCDisableLocalAuth {
		writeError(w, http.StatusForbidden, "L'authentification locale est désactivée : le mot de passe se gère chez votre fournisseur SSO")
		return
	}
	userID := middleware.GetUserID(r.Context())

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	user, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	if user.PasswordHash == nil {
		writeError(w, http.StatusBadRequest, "Ce compte n'a pas de mot de passe local : il se connecte par SSO")
		return
	}

	// Checking the current password is a guessing surface for a stolen session, so it shares the sign-in limit.
	if blocked, retryAfter := h.throttle.Blocked(user.Email); blocked {
		w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
		writeError(w, http.StatusTooManyRequests, "Trop de tentatives : réessayez plus tard")
		return
	}
	if !auth.CheckPassword(req.CurrentPassword, *user.PasswordHash) {
		h.throttle.Fail(user.Email)
		writeError(w, http.StatusForbidden, "Mot de passe actuel incorrect")
		return
	}
	h.throttle.Reset(user.Email)

	switch {
	case len(req.NewPassword) < 8:
		writeError(w, http.StatusBadRequest, "Le nouveau mot de passe doit faire au moins 8 caractères")
		return
	case len(req.NewPassword) > auth.MaxPasswordBytes:
		writeError(w, http.StatusBadRequest, "Le nouveau mot de passe ne doit pas dépasser 72 octets")
		return
	case req.NewPassword == req.CurrentPassword:
		writeError(w, http.StatusBadRequest, "Le nouveau mot de passe doit différer de l'actuel")
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.repo.UpdatePasswordHash(r.Context(), userID, hash); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeError(w, http.StatusUnauthorized, "Unauthorized")
			return
		}
		writeRepoError(w, r, err, "Impossible de modifier le mot de passe")
		return
	}

	// Without the refresh cookie this device cannot be told apart: every session is closed, this one included.
	revoked, err := h.repo.RevokeUserSessionsExcept(r.Context(), userID, h.currentFamily(r))
	if err != nil {
		slog.ErrorContext(r.Context(), "password changed but the other sessions could not be revoked", "component", "auth", "error", err)
		writeError(w, http.StatusInternalServerError, "Mot de passe modifié, mais les autres appareils n'ont pas pu être déconnectés")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "Mot de passe modifié", "sessions_revoked": revoked})
}
