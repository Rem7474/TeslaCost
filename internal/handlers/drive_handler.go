package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

type DriveHandler struct {
	repo *database.Repository
}

func NewDriveHandler(repo *database.Repository) *DriveHandler {
	return &DriveHandler{repo: repo}
}

func (h *DriveHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	// Ensure vehicle belongs to user
	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	tag := r.URL.Query().Get("tag")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	drives, total, err := h.repo.ListDrives(r.Context(), vehicleID, tag, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list drives")
		return
	}
	if drives == nil {
		drives = []models.Drive{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"drives": drives,
		"total":  total,
		"page":   page,
		"limit":  limit,
	})
}

type UpdateTagsRequest struct {
	Tags []string `json:"tags"`
}

func (h *DriveHandler) UpdateTags(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req UpdateTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Tags == nil {
		req.Tags = []string{}
	}

	if err := h.repo.UpdateDriveTags(r.Context(), driveID, vehicleID, req.Tags); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update tags")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"tags":    req.Tags,
	})
}

type CreateTripGroupRequest struct {
	Name     string   `json:"name"`
	Notes    *string  `json:"notes"`
	DriveIDs []string `json:"drive_ids"`
}

func (h *DriveHandler) CreateTripGroup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req CreateTripGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Trip group name is required")
		return
	}

	tg := &models.TripGroup{
		VehicleID: vehicleID,
		Name:      req.Name,
		Notes:     req.Notes,
	}

	if err := h.repo.CreateTripGroup(r.Context(), tg, req.DriveIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create trip group")
		return
	}

	writeJSON(w, http.StatusCreated, tg)
}

func (h *DriveHandler) ListTripGroups(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	groups, err := h.repo.ListTripGroups(r.Context(), vehicleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list trip groups")
		return
	}
	if groups == nil {
		groups = []models.TripGroup{}
	}

	writeJSON(w, http.StatusOK, groups)
}
