package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/models"
)

func (h *TireHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")

	v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer)
	if v == nil {
		return
	}

	t, err := h.repo.GetTireByID(r.Context(), tireID, vehicleID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Tire not found")
		return
	}

	stats, err := h.tireWearService.CalculateTireWear(r.Context(), t, v.CurrentOdometer)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to calculate tire wear", "component", "tire", "tire_id", tireID, "error", err)
	}
	sessions, err := h.repo.ListTireMountSessions(r.Context(), tireID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list mount sessions", "component", "tire", "tire_id", tireID, "error", err)
	}
	logs, err := h.repo.ListTireLogs(r.Context(), tireID)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to list tire logs", "component", "tire", "tire_id", tireID, "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"tire":     t,
		"stats":    stats,
		"sessions": sessions,
		"logs":     logs,
	})
}

type CopyTireHistoryRequest struct {
	TargetTireIDs []string `json:"target_tire_ids"`
	CopySessions  bool     `json:"copy_sessions"`
	CopyLogs      bool     `json:"copy_logs"`
	AdaptPosition bool     `json:"adapt_position"`
}

func (h *TireHandler) CopyHistory(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	var req CopyTireHistoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	if len(req.TargetTireIDs) == 0 {
		writeError(w, http.StatusBadRequest, "Au moins un pneu cible requis (target_tire_ids)")
		return
	}
	sourceTireID := chi.URLParam(r, "tireId")
	if err := h.repo.CopyTireHistory(r.Context(), vehicleID, sourceTireID, req.TargetTireIDs, req.CopySessions, req.CopyLogs, req.AdaptPosition); err != nil {
		writeRepoError(w, r, err, "Failed to copy tire history")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
