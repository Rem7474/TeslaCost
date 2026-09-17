package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

type VehicleMemberHandler struct {
	repo *database.Repository
}

func NewVehicleMemberHandler(repo *database.Repository) *VehicleMemberHandler {
	return &VehicleMemberHandler{repo: repo}
}

func (h *VehicleMemberHandler) getVehicleID(r *http.Request) string {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = chi.URLParam(r, "vehicleId")
	}
	return id
}

// ListMembers lists all members of the specified vehicle.
func (h *VehicleMemberHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := h.getVehicleID(r)

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Véhicule non trouvé")
		return
	}

	members, err := h.repo.ListVehicleMembers(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Impossible de récupérer les membres")
		return
	}
	if members == nil {
		members = []models.VehicleMember{}
	}
	writeJSON(w, http.StatusOK, members)
}

// AddMember adds a user to the vehicle by email. Restricted to OWNER.
func (h *VehicleMemberHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := h.getVehicleID(r)

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Véhicule non trouvé")
		return
	}
	if v.Role != models.RoleOwner {
		writeError(w, http.StatusForbidden, "Seul le propriétaire peut ajouter des membres")
		return
	}

	var req models.AddVehicleMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requête invalide")
		return
	}

	req.Email = strings.TrimSpace(req.Email)
	if req.Email == "" {
		writeError(w, http.StatusBadRequest, "L'adresse email est requise")
		return
	}
	if !req.Role.IsValid() {
		writeError(w, http.StatusBadRequest, "Rôle invalide")
		return
	}

	member, err := h.repo.AddVehicleMember(r.Context(), vehicleID, req.Email, req.Role)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Aucun utilisateur trouvé avec cette adresse email")
			return
		}
		writeRepoError(w, r, err, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, member)
}

// UpdateMemberRole modifies a member's role. Restricted to OWNER.
func (h *VehicleMemberHandler) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := h.getVehicleID(r)
	targetUserID := chi.URLParam(r, "memberId")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Véhicule non trouvé")
		return
	}
	if v.Role != models.RoleOwner {
		writeError(w, http.StatusForbidden, "Seul le propriétaire peut modifier les rôles")
		return
	}

	var req models.UpdateVehicleMemberRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requête invalide")
		return
	}
	if !req.Role.IsValid() {
		writeError(w, http.StatusBadRequest, "Rôle invalide")
		return
	}

	if err := h.repo.UpdateVehicleMemberRole(r.Context(), vehicleID, targetUserID, req.Role); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Membre non trouvé")
			return
		}
		writeRepoError(w, r, err, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Rôle mis à jour avec succès"})
}

// RemoveMember removes a member's access. Restricted to OWNER or the member themselves.
func (h *VehicleMemberHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := h.getVehicleID(r)
	targetUserID := chi.URLParam(r, "memberId")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Véhicule non trouvé")
		return
	}

	if v.Role != models.RoleOwner && targetUserID != userID {
		writeError(w, http.StatusForbidden, "Seul le propriétaire peut retirer un membre (ou vous pouvez vous retirer vous-même)")
		return
	}

	if err := h.repo.RemoveVehicleMember(r.Context(), vehicleID, targetUserID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Membre non trouvé")
			return
		}
		writeRepoError(w, r, err, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Membre retiré avec succès"})
}
