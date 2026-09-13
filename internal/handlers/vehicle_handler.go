package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
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
	AnnualInsuranceCost   *money.Cents    `json:"annual_insurance_cost"`
	AnnualExpectedMileage *float64        `json:"annual_expected_mileage"`
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
		AnnualInsuranceCost:      req.AnnualInsuranceCost,
		AnnualExpectedMileage:    req.AnnualExpectedMileage,
	}

	if err := h.repo.CreateVehicle(r.Context(), v); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create vehicle: "+err.Error())
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

	existing.AnnualInsuranceCost = req.AnnualInsuranceCost
	existing.AnnualExpectedMileage = req.AnnualExpectedMileage

	if err := h.repo.UpdateVehicle(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update vehicle")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (h *VehicleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")

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

	status, err := h.syncService.TestConnection(r.Context(), v)
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

	res, err := h.syncService.SyncVehicle(r.Context(), v)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, res)
}
