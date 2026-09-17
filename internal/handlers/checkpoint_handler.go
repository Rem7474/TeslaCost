package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
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

// decodeCheckpointRequest reads and validates the shared payload used by Create and Update.
func decodeCheckpointRequest(r *http.Request) (time.Time, float64, *string, error) {
	var req SaveCheckpointRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return time.Time{}, 0, nil, errors.New("Invalid request payload")
	}

	d, err := parseDate(req.Date)
	if err != nil {
		return time.Time{}, 0, nil, errors.New("Date invalide")
	}
	if req.Odometer < 0 || req.Odometer > 2_000_000 {
		return time.Time{}, 0, nil, errors.New("Odomètre invalide (doit être compris entre 0 et 2 000 000 km)")
	}

	var notes *string
	if req.Notes != nil && strings.TrimSpace(*req.Notes) != "" {
		n := strings.TrimSpace(*req.Notes)
		notes = &n
	}

	return d, req.Odometer, notes, nil
}

func (h *CheckpointHandler) List(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if vehicleID == "" {
		vehicleID = chi.URLParam(r, "id")
	}

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
	}

	list, err := h.repo.ListOdometerCheckpoints(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list checkpoints")
		return
	}
	if list == nil {
		list = []models.OdometerCheckpoint{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *CheckpointHandler) Create(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if vehicleID == "" {
		vehicleID = chi.URLParam(r, "id")
	}

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	d, odometer, notes, err := decodeCheckpointRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c := &models.OdometerCheckpoint{
		VehicleID: vehicleID,
		Date:      d,
		Odometer:  odometer,
		Notes:     notes,
	}

	if err := h.repo.CreateOdometerCheckpoint(r.Context(), c); err != nil {
		writeRepoError(w, r, err, "Failed to create checkpoint")
		return
	}

	writeJSON(w, http.StatusCreated, c)
}

func (h *CheckpointHandler) Update(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if vehicleID == "" {
		vehicleID = chi.URLParam(r, "id")
	}
	checkpointID := chi.URLParam(r, "checkpointId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	d, odometer, notes, err := decodeCheckpointRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	c := &models.OdometerCheckpoint{
		ID:        checkpointID,
		VehicleID: vehicleID,
		Date:      d,
		Odometer:  odometer,
		Notes:     notes,
	}

	if err := h.repo.UpdateOdometerCheckpoint(r.Context(), c); err != nil {
		writeRepoError(w, r, err, "Failed to update checkpoint")
		return
	}

	writeJSON(w, http.StatusOK, c)
}

func (h *CheckpointHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if vehicleID == "" {
		vehicleID = chi.URLParam(r, "id")
	}
	checkpointID := chi.URLParam(r, "checkpointId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	if err := h.repo.DeleteOdometerCheckpoint(r.Context(), vehicleID, checkpointID); err != nil {
		writeRepoError(w, r, err, "Failed to delete checkpoint")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
