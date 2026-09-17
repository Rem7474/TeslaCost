package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}

// requireVehicleAccess checks that the authenticated user has access to the vehicle with at least minRole.
func requireVehicleAccess(w http.ResponseWriter, r *http.Request, repo *database.Repository, vehicleID string, minRole models.VehicleRole) *models.Vehicle {
	userID := middleware.GetUserID(r.Context())
	v, err := repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return nil
	}
	if minRole == models.RoleOwner && v.Role != models.RoleOwner {
		writeError(w, http.StatusForbidden, "Action réservée au propriétaire du véhicule")
		return nil
	}
	if (minRole == models.RoleEditor || minRole == models.RoleOwner) && !v.Role.CanEdit() {
		writeError(w, http.StatusForbidden, "Accès en lecture seule : modifications non autorisées")
		return nil
	}
	return v
}

