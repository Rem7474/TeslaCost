package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type VehicleHandler struct {
	repo        *database.Repository
	encryptor   *crypto.Encryptor
	syncService *services.SyncService
}

func NewVehicleHandler(repo *database.Repository, encryptor *crypto.Encryptor, syncService *services.SyncService) *VehicleHandler {
	return &VehicleHandler{
		repo:        repo,
		encryptor:   encryptor,
		syncService: syncService,
	}
}

type SaveVehicleRequest struct {
	Name                  string          `json:"name"`
	Vin                   *string         `json:"vin"`
	TeslaMateCarID        *int            `json:"teslamate_car_id"`
	CurrentOdometer       float64         `json:"current_odometer"`
	TeslaMateAPIURL       *string         `json:"teslamate_api_url"`
	TeslaMateAuthType     models.AuthMode `json:"teslamate_auth_type"`
	TeslaMateAPIKey       *string         `json:"teslamate_api_key"` // Plain text from frontend
	TeslaMateBasicUser    *string         `json:"teslamate_basic_user"`
	TeslaMateBasicPass    *string         `json:"teslamate_basic_pass"` // Plain text from frontend
	PreTeslaMateKwh100km  *float64        `json:"pre_teslamate_kwh_100km"`
	PreTeslaMateEurPerKwh *float64        `json:"pre_teslamate_eur_per_kwh"`
}

func (h *VehicleHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	list, err := h.repo.ListVehiclesByUserID(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list vehicles")
		return
	}
	if list == nil {
		list = []models.Vehicle{}
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *VehicleHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())

	var req SaveVehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Vehicle name is required")
		return
	}

	var encKey, encPass *string
	if req.TeslaMateAPIKey != nil && *req.TeslaMateAPIKey != "" {
		encrypted, err := h.encryptor.Encrypt(*req.TeslaMateAPIKey)
		if err == nil {
			encKey = &encrypted
		}
	}
	if req.TeslaMateBasicPass != nil && *req.TeslaMateBasicPass != "" {
		encrypted, err := h.encryptor.Encrypt(*req.TeslaMateBasicPass)
		if err == nil {
			encPass = &encrypted
		}
	}

	authType := req.TeslaMateAuthType
	if authType == "" {
		authType = models.AuthModeNone
	}

	v := &models.Vehicle{
		UserID:                   userID,
		Name:                     req.Name,
		Vin:                      req.Vin,
		TeslaMateCarID:           req.TeslaMateCarID,
		CurrentOdometer:          req.CurrentOdometer,
		TeslaMateAPIURL:          req.TeslaMateAPIURL,
		TeslaMateAuthType:        authType,
		TeslaMateAPIKeyEncrypted: encKey,
		TeslaMateBasicUser:       req.TeslaMateBasicUser,
		TeslaMateBasicPassEnc:    encPass,
		PreTeslaMateKwh100km:     req.PreTeslaMateKwh100km,
		PreTeslaMateEurPerKwh:    req.PreTeslaMateEurPerKwh,
	}

	if err := h.repo.CreateVehicle(r.Context(), v); err != nil {
		writeRepoError(w, r, err, "Failed to create vehicle")
		return
	}

	writeJSON(w, http.StatusCreated, v)
}

func (h *VehicleHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	writeJSON(w, http.StatusOK, v)
}

func (h *VehicleHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	existing, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}
	if existing.Role != models.RoleOwner {
		writeError(w, http.StatusForbidden, "Seul le propriétaire peut modifier la configuration du véhicule")
		return
	}

	var req SaveVehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	existing.Name = req.Name
	existing.Vin = req.Vin
	existing.TeslaMateCarID = req.TeslaMateCarID
	if req.CurrentOdometer > 0 {
		existing.CurrentOdometer = req.CurrentOdometer
	}
	existing.TeslaMateAPIURL = req.TeslaMateAPIURL
	if req.TeslaMateAuthType != "" {
		existing.TeslaMateAuthType = req.TeslaMateAuthType
	}

	if req.TeslaMateAPIKey != nil && *req.TeslaMateAPIKey != "" {
		encrypted, err := h.encryptor.Encrypt(*req.TeslaMateAPIKey)
		if err == nil {
			existing.TeslaMateAPIKeyEncrypted = &encrypted
		}
	}
	if req.TeslaMateBasicUser != nil {
		existing.TeslaMateBasicUser = req.TeslaMateBasicUser
	}
	if req.TeslaMateBasicPass != nil && *req.TeslaMateBasicPass != "" {
		encrypted, err := h.encryptor.Encrypt(*req.TeslaMateBasicPass)
		if err == nil {
			existing.TeslaMateBasicPassEnc = &encrypted
		}
	}
	if req.PreTeslaMateKwh100km != nil {
		existing.PreTeslaMateKwh100km = req.PreTeslaMateKwh100km
	}
	if req.PreTeslaMateEurPerKwh != nil {
		existing.PreTeslaMateEurPerKwh = req.PreTeslaMateEurPerKwh
	}

	if err := h.repo.UpdateVehicle(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update vehicle")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

type SavePreTeslaMateEnergyRequest struct {
	PreTeslaMateKwh100km  *float64 `json:"pre_teslamate_kwh_100km"`
	PreTeslaMateEurPerKwh *float64 `json:"pre_teslamate_eur_per_kwh"`
}

func (h *VehicleHandler) UpdatePreTeslaMateEnergy(w http.ResponseWriter, r *http.Request) {
	var req SavePreTeslaMateEnergyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.PreTeslaMateKwh100km != nil && (*req.PreTeslaMateKwh100km <= 0 || *req.PreTeslaMateKwh100km > 100) {
		writeError(w, http.StatusBadRequest, "La consommation moyenne doit être comprise entre 0 et 100 kWh/100km")
		return
	}
	if req.PreTeslaMateEurPerKwh != nil && (*req.PreTeslaMateEurPerKwh <= 0 || *req.PreTeslaMateEurPerKwh > 10) {
		writeError(w, http.StatusBadRequest, "Le tarif de l'électricité doit être compris entre 0 et 10 €/kWh")
		return
	}

	if h.repo == nil {
		writeJSON(w, http.StatusOK, map[string]any{"success": true})
		return
	}

	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")
	if vehicleID == "" {
		vehicleID = chi.URLParam(r, "vehicleId")
	}

	vCheck, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}
	if !vCheck.Role.CanEdit() {
		writeError(w, http.StatusForbidden, "Droits insuffisants pour modifier ce paramètre")
		return
	}

	if err := h.repo.UpdateVehiclePreTeslaMateEnergy(r.Context(), vehicleID, userID, req.PreTeslaMateKwh100km, req.PreTeslaMateEurPerKwh); err != nil {
		writeRepoError(w, r, err, "Failed to update pre-teslamate energy")
		return
	}

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to get vehicle")
		return
	}

	writeJSON(w, http.StatusOK, v)
}

func (h *VehicleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}
	if v.Role != models.RoleOwner {
		writeError(w, http.StatusForbidden, "Seul le propriétaire peut supprimer le véhicule")
		return
	}

	if err := h.repo.DeleteVehicle(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete vehicle")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Vehicle deleted successfully"})
}

type TestConnectionRequest struct {
	TeslaMateAPIURL    string          `json:"teslamate_api_url"`
	TeslaMateAuthType  models.AuthMode `json:"teslamate_auth_type"`
	TeslaMateAPIKey    *string         `json:"teslamate_api_key"`
	TeslaMateBasicUser *string         `json:"teslamate_basic_user"`
	TeslaMateBasicPass *string         `json:"teslamate_basic_pass"`
	TeslaMateCarID     *int            `json:"teslamate_car_id"`
}

func (h *VehicleHandler) TestTeslaMateRaw(w http.ResponseWriter, r *http.Request) {
	var req TestConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Corps de requête invalide")
		return
	}

	apiKey := ""
	if req.TeslaMateAPIKey != nil {
		apiKey = *req.TeslaMateAPIKey
	}
	basicUser := ""
	if req.TeslaMateBasicUser != nil {
		basicUser = *req.TeslaMateBasicUser
	}
	basicPass := ""
	if req.TeslaMateBasicPass != nil {
		basicPass = *req.TeslaMateBasicPass
	}
	carID := 1
	if req.TeslaMateCarID != nil && *req.TeslaMateCarID > 0 {
		carID = *req.TeslaMateCarID
	}

	status, err := h.syncService.TestConnectionRaw(
		r.Context(),
		req.TeslaMateAPIURL,
		req.TeslaMateAuthType,
		apiKey,
		basicUser,
		basicPass,
		carID,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"status":  status,
	})
}

func (h *VehicleHandler) TestTeslaMate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}
	if v.Role != models.RoleOwner {
		writeError(w, http.StatusForbidden, "Seul le propriétaire peut tester la connexion TeslaMate")
		return
	}

	fullVehicle, err := h.repo.GetVehicleByIDInternal(r.Context(), vehicleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load vehicle credentials")
		return
	}

	status, err := h.syncService.TestConnection(r.Context(), fullVehicle)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"status":  status,
	})
}

func (h *VehicleHandler) Sync(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}
	if v.Role == models.RoleViewer {
		writeError(w, http.StatusForbidden, "Les lecteurs ne peuvent pas déclencher de synchronisation")
		return
	}

	fullVehicle, err := h.repo.GetVehicleByIDInternal(r.Context(), vehicleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to load vehicle credentials")
		return
	}

	job, started := h.syncService.StartSync(*fullVehicle)
	status := http.StatusAccepted
	if !started {
		status = http.StatusOK
	}
	writeJSON(w, status, job)
}

// GetSyncStatus returns the last synchronization job of the vehicle.
func (h *VehicleHandler) GetSyncStatus(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	job := h.syncService.GetSyncJob(vehicleID)
	if job == nil {
		writeJSON(w, http.StatusOK, map[string]any{"vehicle_id": vehicleID, "status": "NONE"})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// GetOdometerAtDate returns the estimated or recorded odometer for the vehicle at a specific date.
func (h *VehicleHandler) GetOdometerAtDate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	dateStr := r.URL.Query().Get("date")
	targetTime := time.Now()
	if dateStr != "" {
		parsed, err := parseDate(dateStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Date invalide")
			return
		}
		// If only date was provided (00:00:00), check drives up to end of that day
		if parsed.Hour() == 0 && parsed.Minute() == 0 && parsed.Second() == 0 {
			targetTime = parsed.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
		} else {
			targetTime = parsed
		}
	}

	odo, source, err := h.repo.GetOdometerAtDate(r.Context(), vehicleID, targetTime)
	if err != nil {
		writeRepoError(w, r, err, "Failed to resolve odometer")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"odometer": odo,
		"source":   source,
	})
}

