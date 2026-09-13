package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/services"
)

type DriveCostBreakdown struct {
	ElectricityCost       money.Cents `json:"electricity_cost"`
	ElectricityKwh        float64     `json:"electricity_kwh"`
	ElectricityRate       float64     `json:"electricity_rate"`
	EnergySource          string      `json:"energy_source"`           // MEASURED | CONSUMPTION | DEFAULT
	ElectricityRateSource string      `json:"electricity_rate_source"` // HISTORY | DEFAULT
	TiresCost             money.Cents `json:"tires_cost"`
	TiresRate             float64     `json:"tires_rate"`
	TiresRateSource       string      `json:"tires_rate_source"`
	MaintenanceCost       money.Cents `json:"maintenance_cost"`
	MaintenanceRate       float64     `json:"maintenance_rate"`
	MaintenanceRateSource string      `json:"maintenance_rate_source"`
	InsuranceCost         money.Cents `json:"insurance_cost"`
	InsuranceRate         float64     `json:"insurance_rate"`
	InsuranceSource       string      `json:"insurance_source"`
	TollsCost             money.Cents `json:"tolls_cost"` // Direct expenses + share of trip group expenses
	TotalCost             money.Cents `json:"total_cost"`
	CostPerKm             float64     `json:"cost_per_km"`
	// HasEstimates is true when at least one component relies on a default assumption.
	HasEstimates bool `json:"has_estimates"`
}

type EnrichedDrive struct {
	models.Drive
	Costs DriveCostBreakdown `json:"costs"`
}

type DriveHandler struct {
	repo           *database.Repository
	carpoolService *services.CarpoolService
}

func NewDriveHandler(repo *database.Repository, carpoolService *services.CarpoolService) *DriveHandler {
	return &DriveHandler{
		repo:           repo,
		carpoolService: carpoolService,
	}
}

func (h *DriveHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	// Ensure vehicle belongs to user
	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	filter := database.DriveFilter{
		Tag:             r.URL.Query().Get("tag"),
		UnqualifiedOnly: r.URL.Query().Get("unqualified") == "true",
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	drives, total, err := h.repo.ListDrives(r.Context(), vehicleID, filter, limit, offset)
	if err != nil {
		writeRepoError(w, err, "Failed to list drives")
		return
	}
	if drives == nil {
		drives = []models.Drive{}
	}

	// Calculate unit rates for real cost breakdown
	rates, err := h.carpoolService.GetVehicleUnitRates(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, err, "Failed to compute cost rates")
		return
	}

	// Fetch toll expenses attached to these drives
	driveIDs := make([]string, len(drives))
	for i, d := range drives {
		driveIDs[i] = d.ID
	}
	tollsMap, err := h.repo.GetTollExpensesForDrives(r.Context(), vehicleID, driveIDs)
	if err != nil {
		writeRepoError(w, err, "Failed to load drive expenses")
		return
	}
	unqualifiedCount, err := h.repo.CountUnqualifiedDrives(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, err, "Failed to list drives")
		return
	}

	enriched := make([]EnrichedDrive, len(drives))
	for i, d := range drives {
		kwh, energySource := services.DriveEnergyKwh(d.DistanceKm, d.EnergyConsumedKwh, d.ConsumptionKwh100km)
		hasEstimates := energySource == services.EnergySourceDefault ||
			rates.ElectricitySource == services.RateSourceDefault ||
			rates.TiresSource == services.RateSourceDefault ||
			rates.MaintenanceSource == services.RateSourceDefault ||
			rates.InsuranceSource == services.InsuranceSourceDefault

		elecCost := money.FromFloat(kwh * rates.ElectricityPerKwh)
		tiresCost := money.FromFloat(d.DistanceKm * rates.TiresPerKm)
		maintCost := money.FromFloat(d.DistanceKm * rates.MaintenancePerKm)
		insCost := money.FromFloat(d.DistanceKm * rates.InsurancePerKm)
		tollsCost := tollsMap[d.ID]
		totalCost := elecCost + tiresCost + maintCost + insCost + tollsCost

		costPerKm := 0.0
		if d.DistanceKm > 0 {
			costPerKm = math.Round((totalCost.Float()/d.DistanceKm)*1000) / 1000
		}

		enriched[i] = EnrichedDrive{
			Drive: d,
			Costs: DriveCostBreakdown{
				ElectricityCost:       elecCost,
				ElectricityKwh:        math.Round(kwh*10) / 10,
				ElectricityRate:       rates.ElectricityPerKwh,
				EnergySource:          energySource,
				ElectricityRateSource: rates.ElectricitySource,
				TiresCost:             tiresCost,
				TiresRate:             rates.TiresPerKm,
				TiresRateSource:       rates.TiresSource,
				MaintenanceCost:       maintCost,
				MaintenanceRate:       rates.MaintenancePerKm,
				MaintenanceRateSource: rates.MaintenanceSource,
				InsuranceCost:         insCost,
				InsuranceRate:         rates.InsurancePerKm,
				InsuranceSource:       rates.InsuranceSource,
				TollsCost:             tollsCost,
				TotalCost:             totalCost,
				CostPerKm:             costPerKm,
				HasEstimates:          hasEstimates,
			},
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"drives":            enriched,
		"total":             total,
		"page":              page,
		"limit":             limit,
		"unqualified_count": unqualifiedCount,
	})
}

func (h *DriveHandler) GetDriveExpenses(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	expenses, err := h.repo.GetDriveExpensesByDriveID(r.Context(), vehicleID, driveID)
	if err != nil {
		writeRepoError(w, err, "Failed to load drive expenses")
		return
	}
	if expenses == nil {
		expenses = []models.DriveExpense{}
	}

	writeJSON(w, http.StatusOK, expenses)
}

type UpdateTagsRequest struct {
	Tags []string `json:"tags"`
}

func (h *DriveHandler) UpdateTags(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req UpdateTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Tags == nil {
		req.Tags = []string{}
	}

	if err := h.repo.UpdateDriveTags(r.Context(), driveID, vehicleID, req.Tags); err != nil {
		writeRepoError(w, err, "Failed to update tags")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"tags":    req.Tags,
	})
}

type TollReviewRequest struct {
	Reviewed bool `json:"reviewed"`
}

// SetTollReview marks a drive as reviewed without toll (or reopens it).
func (h *DriveHandler) SetTollReview(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	driveID := chi.URLParam(r, "driveId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req TollReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.repo.SetDriveTollReviewed(r.Context(), driveID, vehicleID, req.Reviewed); err != nil {
		writeRepoError(w, err, "Failed to update toll review")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "reviewed": req.Reviewed})
}

type CreateTripGroupRequest struct {
	Name     string   `json:"name"`
	Notes    *string  `json:"notes"`
	DriveIDs []string `json:"drive_ids"`
}

func (h *DriveHandler) CreateTripGroup(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req CreateTripGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "Trip group name is required")
		return
	}

	tg := &models.TripGroup{
		VehicleID: vehicleID,
		Name:      req.Name,
		Notes:     req.Notes,
	}

	if len(req.DriveIDs) == 0 {
		writeError(w, http.StatusBadRequest, "Un groupe doit contenir au moins un trajet")
		return
	}

	if err := h.repo.CreateTripGroup(r.Context(), tg, req.DriveIDs); err != nil {
		writeRepoError(w, err, "Failed to create trip group")
		return
	}

	writeJSON(w, http.StatusCreated, tg)
}

func (h *DriveHandler) ListTripGroups(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	groups, err := h.repo.ListTripGroups(r.Context(), vehicleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list trip groups")
		return
	}
	if groups == nil {
		groups = []models.TripGroup{}
	}

	writeJSON(w, http.StatusOK, groups)
}
