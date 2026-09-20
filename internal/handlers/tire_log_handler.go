package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/models"
)

type AddTireLogRequest struct {
	DepthMm  float64 `json:"depth_mm"`
	Odometer float64 `json:"odometer"`
	Notes    *string `json:"notes"`
	Date     string  `json:"date"`
}

func (h *TireHandler) AddLog(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	if err := h.repo.EnsureTireOwned(r.Context(), vehicleID, tireID); err != nil {
		writeRepoError(w, r, err, "Failed to record tire log")
		return
	}

	var req AddTireLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.DepthMm <= 0 || req.DepthMm > 20 {
		writeError(w, http.StatusBadRequest, "La profondeur doit être comprise entre 0 et 20 mm")
		return
	}
	if req.Odometer <= 0 {
		writeError(w, http.StatusBadRequest, "Le relevé d'usure requiert l'odomètre du véhicule")
		return
	}

	logDate := time.Now()
	if req.Date != "" {
		parsed, err := parseDate(req.Date)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		logDate = parsed
	}

	log := &models.TireLog{
		TireID:   tireID,
		Date:     logDate,
		Odometer: req.Odometer,
		DepthMm:  req.DepthMm,
		Notes:    req.Notes,
	}

	if err := h.repo.AddTireLog(r.Context(), log); err != nil {
		writeRepoError(w, r, err, "Failed to record tire log")
		return
	}

	writeJSON(w, http.StatusCreated, log)
}

// UpdateLog corrects a tread depth measurement.
func (h *TireHandler) UpdateLog(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	var req AddTireLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	if req.DepthMm <= 0 || req.DepthMm > 20 || req.Odometer <= 0 {
		writeError(w, http.StatusBadRequest, "Profondeur (0 à 20 mm) et odomètre requis")
		return
	}
	date, err := parseDate(req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	l := &models.TireLog{ID: chi.URLParam(r, "logId"), TireID: chi.URLParam(r, "tireId"), Date: date, Odometer: req.Odometer, DepthMm: req.DepthMm, Notes: req.Notes}
	if err := h.repo.UpdateTireLog(r.Context(), vehicleID, l); err != nil {
		writeRepoError(w, r, err, "Failed to update tire log")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

// DeleteLog deletes a tread depth measurement.
func (h *TireHandler) DeleteLog(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	if err := h.repo.DeleteTireLog(r.Context(), vehicleID, chi.URLParam(r, "tireId"), chi.URLParam(r, "logId")); err != nil {
		writeRepoError(w, r, err, "Failed to delete tire log")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
