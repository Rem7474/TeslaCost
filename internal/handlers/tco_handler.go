package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

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
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	summary, err := h.tcoService.ComputeVehicleTCO(r.Context(), vehicleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to compute TCO: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summary)
}
