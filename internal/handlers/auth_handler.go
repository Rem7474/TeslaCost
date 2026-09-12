package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/teslacost/teslacost/internal/auth"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
)

type AuthHandler struct {
	repo                *database.Repository
	jwtSecret           string
	jwtExpirationHours  int
	disableRegistration bool
}

func NewAuthHandler(repo *database.Repository, jwtSecret string, jwtExpirationHours int, disableRegistration bool) *AuthHandler {
	return &AuthHandler{
		repo:                repo,
		jwtSecret:           jwtSecret,
		jwtExpirationHours:  jwtExpirationHours,
		disableRegistration: disableRegistration,
	}
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

// GetConfig returns public authentication configuration (e.g. whether registration is open or onboarding is required)
func (h *AuthHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	count, err := h.repo.GetUserCount(r.Context())
	userCount := 0
	if err == nil {
		userCount = count
	}
	needsOnboarding := (userCount == 0)

	regEnabled := !h.disableRegistration
	if needsOnboarding {
		regEnabled = true
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"registration_enabled": regEnabled,
		"needs_onboarding":     needsOnboarding,
		"user_count":           userCount,
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	if h.disableRegistration {
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

	token, err := auth.GenerateToken(user.ID, user.Email, h.jwtSecret, h.jwtExpirationHours)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	writeJSON(w, http.StatusCreated, AuthResponse{
		Token: token,
		User:  user,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := h.repo.GetUserByEmail(r.Context(), req.Email)
	if err != nil || !auth.CheckPassword(req.Password, user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Email, h.jwtSecret, h.jwtExpirationHours)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	writeJSON(w, http.StatusOK, AuthResponse{
		Token: token,
		User:  user,
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
