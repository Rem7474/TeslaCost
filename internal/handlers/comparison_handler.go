package handlers

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

// ComparisonHandler serves the informational EV vs ICE cost comparison.
type ComparisonHandler struct {
	repo    *database.Repository
	service *services.ComparisonService
}

func NewComparisonHandler(repo *database.Repository, service *services.ComparisonService) *ComparisonHandler {
	return &ComparisonHandler{repo: repo, service: service}
}

// SaveComparisonRequest is the payload shared by Create and Update.
type SaveComparisonRequest struct {
	VehicleID *string                `json:"vehicle_id"`
	Name      string                 `json:"name"`
	Mode      string                 `json:"mode"`
	AnnualKm  float64                `json:"annual_km"`
	Years     int                    `json:"years"`
	ICE       models.ICEInputs       `json:"ice"`
	EV        *models.EVInputs       `json:"ev"`
	Options   models.ScenarioOptions `json:"options"`
}

func validateRange(v, min, max float64, message string) error {
	if math.IsNaN(v) || math.IsInf(v, 0) || v < min || v > max {
		return errors.New(message)
	}
	return nil
}

func validateAmounts(amounts ...money.Cents) error {
	for _, a := range amounts {
		if err := validateAmount(a, true); err != nil {
			return err
		}
	}
	return nil
}

// validateComparisonRequest checks the bounds of every input and normalizes the scenario name and links.
func validateComparisonRequest(req *SaveComparisonRequest) error {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 100 {
		return errors.New("Nom invalide (1 à 100 caractères)")
	}
	if req.Mode != models.ComparisonModeRetrospective && req.Mode != models.ComparisonModeProjection {
		return errors.New("Mode invalide (RETROSPECTIVE ou PROJECTION)")
	}
	if err := validateRange(req.AnnualKm, 1, 200_000, "Kilométrage annuel invalide (1 à 200 000 km)"); err != nil {
		return err
	}
	if req.Years < 1 || req.Years > 15 {
		return errors.New("Durée invalide (1 à 15 ans)")
	}

	ice := req.ICE
	if !models.FuelTypes[ice.FuelType] {
		return errors.New("Carburant invalide")
	}
	if err := validateRange(ice.LPer100Km, 0.1, 50, "Consommation thermique invalide (0,1 à 50 L/100 km)"); err != nil {
		return err
	}
	if err := validateRange(ice.FuelPrice, 0, 10, "Prix du carburant invalide (0 à 10 €/L)"); err != nil {
		return err
	}
	if err := validateAmounts(ice.PurchasePrice, ice.ResaleValue, ice.MaintenanceYearly, ice.InsuranceYearly, ice.TaxYearly); err != nil {
		return err
	}

	for _, pct := range []float64{req.Options.FuelInflationPct, req.Options.ElectricityInflationPct, req.Options.CostInflationPct} {
		if err := validateRange(pct, -10, 30, "Inflation invalide (−10 à 30 %)"); err != nil {
			return err
		}
	}
	if err := validateAmounts(req.Options.EVIncentives); err != nil {
		return err
	}

	switch req.Mode {
	case models.ComparisonModeRetrospective:
		if req.VehicleID == nil || *req.VehicleID == "" {
			return errors.New("Un véhicule est requis en mode rétrospectif")
		}
		req.EV = nil
	case models.ComparisonModeProjection:
		req.VehicleID = nil
		if req.EV == nil {
			return errors.New("Les données du véhicule électrique sont requises en mode projection")
		}
		if err := validateRange(req.EV.KwhPer100Km, 0.1, 100, "Consommation électrique invalide (0,1 à 100 kWh/100 km)"); err != nil {
			return err
		}
		if err := validateRange(req.EV.EurPerKwh, 0, 5, "Prix de l'électricité invalide (0 à 5 €/kWh)"); err != nil {
			return err
		}
		if err := validateAmounts(req.EV.PurchasePrice, req.EV.ResaleValue, req.EV.MaintenanceYearly, req.EV.InsuranceYearly, req.EV.TaxYearly); err != nil {
			return err
		}
	}
	return nil
}

func (r *SaveComparisonRequest) toScenario(userID string) *models.ComparisonScenario {
	return &models.ComparisonScenario{
		UserID:    userID,
		VehicleID: r.VehicleID,
		Name:      r.Name,
		Mode:      r.Mode,
		AnnualKm:  r.AnnualKm,
		Years:     r.Years,
		ICE:       r.ICE,
		EV:        r.EV,
		Options:   r.Options,
	}
}

// decodeAndCheck reads the request, validates it and checks the access to the reference vehicle.
func (h *ComparisonHandler) decodeAndCheck(w http.ResponseWriter, r *http.Request) (*SaveComparisonRequest, bool) {
	var req SaveComparisonRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return nil, false
	}
	if err := validateComparisonRequest(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return nil, false
	}
	if req.VehicleID != nil {
		v := requireVehicleAccess(w, r, h.repo, *req.VehicleID, models.RoleViewer)
		if v == nil {
			return nil, false
		}
		if v.Powertrain == models.PowertrainICE {
			writeError(w, http.StatusBadRequest, "Le comparatif « véhicule suivi » s'appuie sur un véhicule électrique ; utilisez le mode projection")
			return nil, false
		}
	}
	return &req, true
}

func (h *ComparisonHandler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.ListComparisonScenarios(r.Context(), middleware.GetUserID(r.Context()))
	if err != nil {
		writeRepoError(w, r, err, "Failed to list comparisons")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *ComparisonHandler) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeAndCheck(w, r)
	if !ok {
		return
	}
	sc := req.toScenario(middleware.GetUserID(r.Context()))
	if err := h.repo.CreateComparisonScenario(r.Context(), sc); err != nil {
		writeRepoError(w, r, err, "Failed to create comparison")
		return
	}
	writeJSON(w, http.StatusCreated, sc)
}

func (h *ComparisonHandler) Update(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeAndCheck(w, r)
	if !ok {
		return
	}
	sc := req.toScenario(middleware.GetUserID(r.Context()))
	sc.ID = chi.URLParam(r, "scenarioId")
	if err := h.repo.UpdateComparisonScenario(r.Context(), sc); err != nil {
		writeRepoError(w, r, err, "Failed to update comparison")
		return
	}
	writeJSON(w, http.StatusOK, sc)
}

func (h *ComparisonHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.DeleteComparisonScenario(r.Context(), middleware.GetUserID(r.Context()), chi.URLParam(r, "scenarioId")); err != nil {
		writeRepoError(w, r, err, "Failed to delete comparison")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// Result evaluates a saved scenario. Nothing is written.
func (h *ComparisonHandler) Result(w http.ResponseWriter, r *http.Request) {
	sc, err := h.repo.GetComparisonScenario(r.Context(), middleware.GetUserID(r.Context()), chi.URLParam(r, "scenarioId"))
	if err != nil {
		writeRepoError(w, r, err, "Failed to load comparison")
		return
	}
	if sc.VehicleID != nil && requireVehicleAccess(w, r, h.repo, *sc.VehicleID, models.RoleViewer) == nil {
		return
	}
	res, err := h.service.Compare(r.Context(), sc)
	if err != nil {
		writeRepoError(w, r, err, "Failed to compute comparison")
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// Defaults returns the form prefill, using the vehicle's real data when vehicle_id is given.
func (h *ComparisonHandler) Defaults(w http.ResponseWriter, r *http.Request) {
	vehicleID := r.URL.Query().Get("vehicle_id")
	if vehicleID != "" && requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer) == nil {
		return
	}
	d, err := h.service.Defaults(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to compute defaults")
		return
	}
	writeJSON(w, http.StatusOK, d)
}
