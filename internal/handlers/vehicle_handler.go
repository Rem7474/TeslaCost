package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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
	AcquisitionType       *string         `json:"acquisition_type"`
	PurchasePrice         *money.Cents    `json:"purchase_price"`
	PurchaseDate          *string         `json:"purchase_date"`
	PurchaseOdometer      *float64        `json:"purchase_odometer"`
	PurchaseIncentives    *money.Cents    `json:"purchase_incentives"`
	ExpectedResaleValue   *money.Cents    `json:"expected_resale_value"`
	ExpectedHoldingMonths *int            `json:"expected_holding_months"`
}

// applyAcquisition validates and copies the acquisition settings of a vehicle payload.
func applyAcquisition(v *models.Vehicle, req *SaveVehicleRequest) error {
	v.AcquisitionType, v.PurchasePrice, v.PurchaseDate, v.PurchaseOdometer = nil, nil, nil, nil
	v.PurchaseIncentives, v.ExpectedResaleValue, v.ExpectedHoldingMonths = nil, nil, nil
	if req.AcquisitionType == nil || *req.AcquisitionType == "" {
		return nil
	}
	acqType := strings.ToUpper(*req.AcquisitionType)
	if acqType != "PURCHASE" && acqType != "LEASE" {
		return errors.New("type d'acquisition invalide (PURCHASE ou LEASE)")
	}
	v.AcquisitionType = &acqType

	date, err := parseOptionalDate(req.PurchaseDate)
	if err != nil {
		return err
	}
	v.PurchaseDate = date
	if req.PurchaseOdometer != nil {
		if err := validateQuantity(*req.PurchaseOdometer, 2_000_000); err != nil {
			return errors.New("odomètre d'acquisition invalide")
		}
		v.PurchaseOdometer = req.PurchaseOdometer
	}
	if acqType == "LEASE" {
		return nil
	}

	for _, amount := range []*money.Cents{req.PurchasePrice, req.PurchaseIncentives, req.ExpectedResaleValue} {
		if amount != nil {
			if err := validateAmount(*amount, true); err != nil {
				return err
			}
		}
	}
	if req.PurchasePrice == nil || *req.PurchasePrice == 0 || date == nil {
		return errors.New("un achat requiert un prix et une date d'acquisition")
	}
	net := *req.PurchasePrice
	if req.PurchaseIncentives != nil {
		net -= *req.PurchaseIncentives
	}
	if req.ExpectedResaleValue != nil && *req.ExpectedResaleValue > net {
		return errors.New("la valeur de revente dépasse le prix d'achat net des aides")
	}
	if req.ExpectedHoldingMonths != nil && (*req.ExpectedHoldingMonths <= 0 || *req.ExpectedHoldingMonths > 360) {
		return errors.New("la durée de détention doit être comprise entre 1 et 360 mois")
	}
	v.PurchasePrice = req.PurchasePrice
	v.PurchaseIncentives = req.PurchaseIncentives
	v.ExpectedResaleValue = req.ExpectedResaleValue
	v.ExpectedHoldingMonths = req.ExpectedHoldingMonths
	return nil
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
	if err := applyAcquisition(v, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.repo.CreateVehicle(r.Context(), v); err != nil {
		writeRepoError(w, err, "Failed to create vehicle")
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
	if err := applyAcquisition(existing, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

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
