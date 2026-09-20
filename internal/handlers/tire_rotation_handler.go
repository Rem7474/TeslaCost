package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
)

type QuickRotateRequest struct {
	Mode                string   `json:"mode"` // "FRONT_BACK", "CROSS", "SWAP_PACK"
	Odometer            float64  `json:"odometer"`
	SwapWithPackTireIDs []string `json:"swap_with_pack_tire_ids"`
}

func (h *TireHandler) QuickRotate(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req QuickRotateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	if err := h.repo.QuickRotateTires(r.Context(), vehicleID, req.Mode, req.Odometer, req.SwapWithPackTireIDs); err != nil {
		writeRepoError(w, r, err, "Failed to perform quick rotation")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

type RotationRequest struct {
	Date        string         `json:"date"`
	Odometer    float64        `json:"odometer"`
	MappingJSON map[string]any `json:"mapping_json"`
	Notes       *string        `json:"notes"`
}

func (h *TireHandler) Rotate(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req RotationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAPIError(w, http.StatusBadRequest, apierror.New("request.invalid_body", "Invalid request body"))
		return
	}

	rotDate := time.Now().UTC()
	if req.Date != "" {
		parsed, err := parseDate(req.Date)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		rotDate = parsed
	}

	rot := &models.TireRotation{
		VehicleID:   vehicleID,
		Date:        rotDate,
		Odometer:    req.Odometer,
		MappingJSON: req.MappingJSON,
		Notes:       req.Notes,
	}

	if err := h.repo.AddTireRotation(r.Context(), rot); err != nil {
		writeRepoError(w, r, err, "Failed to apply tire rotation")
		return
	}

	writeJSON(w, http.StatusCreated, rot)
}
