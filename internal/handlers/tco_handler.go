package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/services"
)

type TCOHandler struct {
	repo       *database.Repository
	tcoService *services.TCOService
}

func NewTCOHandler(repo *database.Repository, tcoService *services.TCOService) *TCOHandler {
	return &TCOHandler{
		repo:       repo,
		tcoService: tcoService,
	}
}

func (h *TCOHandler) GetTCO(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}

	summary, err := h.tcoService.ComputeVehicleTCO(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to compute TCO")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// GetDataQuality lists odometer continuity issues of the vehicle drives.
func (h *TCOHandler) GetDataQuality(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeAPIError(w, http.StatusNotFound, apierror.New("vehicle.not_found", "Vehicle not found"))
		return
	}

	issues, err := h.repo.ListDataQualityIssues(r.Context(), vehicleID, 100)
	if err != nil {
		writeRepoError(w, r, err, "Failed to check data quality")
		return
	}
	gaps, gapKm, anomalies, err := h.repo.DataQualitySummary(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to check data quality")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"issues":             issues,
		"odometer_gaps":      gaps,
		"odometer_gap_km":    gapKm,
		"odometer_anomalies": anomalies,
	})
}
