package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

type CheckpointHandler struct {
	repo *database.Repository
}

func NewCheckpointHandler(repo *database.Repository) *CheckpointHandler {
	return &CheckpointHandler{repo: repo}
}

type SaveCheckpointRequest struct {
	Date     string  `json:"date"`
	Odometer float64 `json:"odometer"`
	Notes    *string `json:"notes"`
}

func (h *CheckpointHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	if vehicleID == "" {
		vehicleID = chi.URLParam(r, "id")
	}

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	list, err := h.repo.ListOdometerCheckpoints(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, err, "Failed to list checkpoints")
		return
	}
	if list == nil {
		list = []models.OdometerCheckpoint{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *CheckpointHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	if vehicleID == "" {
		vehicleID = chi.URLParam(r, "id")
	}

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req SaveCheckpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	d, err := parseDate(req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Date invalide")
		return
	}
	if req.Odometer < 0 || req.Odometer > 2_000_000 {
		writeError(w, http.StatusBadRequest, "Odomètre invalide (doit être compris entre 0 et 2 000 000 km)")
		return
	}

	var notes *string
	if req.Notes != nil && strings.TrimSpace(*req.Notes) != "" {
		n := strings.TrimSpace(*req.Notes)
		notes = &n
	}

	c := &models.OdometerCheckpoint{
		VehicleID: vehicleID,
		Date:      d,
		Odometer:  req.Odometer,
		Notes:     notes,
	}

	if err := h.repo.CreateOdometerCheckpoint(r.Context(), c); err != nil {
		writeRepoError(w, err, "Failed to create checkpoint")
		return
	}

	writeJSON(w, http.StatusCreated, c)
}

func (h *CheckpointHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	if vehicleID == "" {
		vehicleID = chi.URLParam(r, "id")
	}
	checkpointID := chi.URLParam(r, "checkpointId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req SaveCheckpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	d, err := parseDate(req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Date invalide")
		return
	}
	if req.Odometer < 0 || req.Odometer > 2_000_000 {
		writeError(w, http.StatusBadRequest, "Odomètre invalide (doit être compris entre 0 et 2 000 000 km)")
		return
	}

	var notes *string
	if req.Notes != nil && strings.TrimSpace(*req.Notes) != "" {
		n := strings.TrimSpace(*req.Notes)
		notes = &n
	}

	c := &models.OdometerCheckpoint{
		ID:        checkpointID,
		VehicleID: vehicleID,
		Date:      d,
		Odometer:  req.Odometer,
		Notes:     notes,
	}

	if err := h.repo.UpdateOdometerCheckpoint(r.Context(), c); err != nil {
		writeRepoError(w, err, "Failed to update checkpoint")
		return
	}

	writeJSON(w, http.StatusOK, c)
}

func (h *CheckpointHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	if vehicleID == "" {
		vehicleID = chi.URLParam(r, "id")
	}
	checkpointID := chi.URLParam(r, "checkpointId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	if err := h.repo.DeleteOdometerCheckpoint(r.Context(), vehicleID, checkpointID); err != nil {
		writeRepoError(w, err, "Failed to delete checkpoint")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
