package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/models"
)

type MountSessionPayload struct {
	Position           models.TirePosition `json:"position"`
	MountedDate        string              `json:"mounted_date"`
	MountedOdometer    float64             `json:"mounted_odometer"`
	DismountedDate     *string             `json:"dismounted_date"`
	DismountedOdometer *float64            `json:"dismounted_odometer"`
	DistanceKm         float64             `json:"distance_km"`
	Notes              *string             `json:"notes"`
}

func parseSessionPayload(req *MountSessionPayload) (time.Time, *time.Time, error) {
	// Historical (completed) sessions may be stored without a precise wheel position.
	// In that case the frontend sends position=STORAGE, which is accepted only when
	// a dismounted_date is provided (i.e. the session is already over).
	isHistoricalStorage := req.Position == models.TirePosStorage && req.DismountedDate != nil
	if !isMountedPosition(req.Position) && !isHistoricalStorage {
		return time.Time{}, nil, errors.New("une session de montage requiert une position FL, FR, RL ou RR (ou STORAGE pour une session historique terminée)")
	}
	if req.MountedOdometer < 0 || (req.DismountedOdometer != nil && *req.DismountedOdometer < 0) {
		return time.Time{}, nil, errors.New("odomètre invalide")
	}
	mountedDate, err := parseDate(req.MountedDate)
	if err != nil {
		return time.Time{}, nil, err
	}
	dismountedDate, err := parseOptionalDate(req.DismountedDate)
	if err != nil {
		return time.Time{}, nil, err
	}
	if dismountedDate != nil && dismountedDate.Before(mountedDate) {
		return time.Time{}, nil, errors.New("la date de démontage précède la date de montage")
	}
	return mountedDate, dismountedDate, nil
}

// decodeMountSession reads a mount session of the given tire from the request body. It writes the error response
// and returns false when the payload is invalid.
func decodeMountSession(w http.ResponseWriter, r *http.Request, vehicleID, tireID string) (*models.TireMountSession, bool) {
	var req MountSessionPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return nil, false
	}

	mountedDate, dismountedDate, err := parseSessionPayload(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return nil, false
	}

	return &models.TireMountSession{
		TireID:             tireID,
		VehicleID:          vehicleID,
		Position:           req.Position,
		MountedDate:        mountedDate,
		MountedOdometer:    req.MountedOdometer,
		DismountedDate:     dismountedDate,
		DismountedOdometer: req.DismountedOdometer,
		DistanceKm:         req.DistanceKm,
		Notes:              req.Notes,
	}, true
}

func (h *TireHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	session, ok := decodeMountSession(w, r, vehicleID, tireID)
	if !ok {
		return
	}

	if err := h.repo.CreateTireMountSession(r.Context(), session); err != nil {
		writeRepoError(w, r, err, "Failed to create mount session")
		return
	}

	writeJSON(w, http.StatusCreated, session)
}

func (h *TireHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")
	sessionID := chi.URLParam(r, "sessionId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	session, ok := decodeMountSession(w, r, vehicleID, tireID)
	if !ok {
		return
	}
	session.ID = sessionID

	if err := h.repo.UpdateTireMountSession(r.Context(), session); err != nil {
		writeRepoError(w, r, err, "Failed to update mount session")
		return
	}

	writeJSON(w, http.StatusOK, session)
}

func (h *TireHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")
	sessionID := chi.URLParam(r, "sessionId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	if err := h.repo.DeleteTireMountSession(r.Context(), vehicleID, sessionID, tireID); err != nil {
		writeRepoError(w, r, err, "Failed to delete mount session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
