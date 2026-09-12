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
	"github.com/teslacost/teslacost/internal/services"
)

type DriveCostBreakdown struct {
	ElectricityCost float64 `json:"electricity_cost"`
	ElectricityKwh  float64 `json:"electricity_kwh"`
	ElectricityRate float64 `json:"electricity_rate"`
	TiresCost       float64 `json:"tires_cost"`
	TiresRate       float64 `json:"tires_rate"`
	MaintenanceCost float64 `json:"maintenance_cost"`
	MaintenanceRate float64 `json:"maintenance_rate"`
	InsuranceCost   float64 `json:"insurance_cost"`
	InsuranceRate   float64 `json:"insurance_rate"`
	TollsCost       float64 `json:"tolls_cost"`
	TotalCost       float64 `json:"total_cost"`
	CostPerKm       float64 `json:"cost_per_km"`
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

	tag := r.URL.Query().Get("tag")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	drives, total, err := h.repo.ListDrives(r.Context(), vehicleID, tag, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list drives")
		return
	}
	if drives == nil {
		drives = []models.Drive{}
	}

	// Calculate unit rates for real cost breakdown
	rates, _ := h.carpoolService.GetVehicleUnitRates(r.Context(), vehicleID)
	if rates == nil {
		rates = &services.UnitRates{
			ElectricityPerKwh: 0.22,
			TiresPerKm:        0.020,
			MaintenancePerKm:  0.015,
			InsurancePerKm:    0.035,
		}
	}

	// Fetch toll expenses attached to these drives
	driveIDs := make([]string, len(drives))
	for i, d := range drives {
		driveIDs[i] = d.ID
	}
	tollsMap, _ := h.repo.GetTollExpensesForDrives(r.Context(), vehicleID, driveIDs)
	if tollsMap == nil {
		tollsMap = make(map[string]float64)
	}

	enriched := make([]EnrichedDrive, len(drives))
	for i, d := range drives {
		kwh := 0.0
		if d.EnergyConsumedKwh != nil && *d.EnergyConsumedKwh > 0 {
			kwh = *d.EnergyConsumedKwh
		} else if d.ConsumptionKwh100km != nil && *d.ConsumptionKwh100km > 0 {
			kwh = (d.DistanceKm * *d.ConsumptionKwh100km) / 100.0
		} else if d.DistanceKm > 0 {
			kwh = (d.DistanceKm * 16.5) / 100.0
		}

		elecCost := math.Round(kwh*rates.ElectricityPerKwh*100) / 100
		tiresCost := math.Round(d.DistanceKm*rates.TiresPerKm*100) / 100
		maintCost := math.Round(d.DistanceKm*rates.MaintenancePerKm*100) / 100
		insCost := math.Round(d.DistanceKm*rates.InsurancePerKm*100) / 100
		tollsCost := math.Round(tollsMap[d.ID]*100) / 100
		totalCost := math.Round((elecCost+tiresCost+maintCost+insCost+tollsCost)*100) / 100

		costPerKm := 0.0
		if d.DistanceKm > 0 {
			costPerKm = math.Round((totalCost/d.DistanceKm)*1000) / 1000
		}

		enriched[i] = EnrichedDrive{
			Drive: d,
			Costs: DriveCostBreakdown{
				ElectricityCost: elecCost,
				ElectricityKwh:  math.Round(kwh*10) / 10,
				ElectricityRate: rates.ElectricityPerKwh,
				TiresCost:       tiresCost,
				TiresRate:       rates.TiresPerKm,
				MaintenanceCost: maintCost,
				MaintenanceRate: rates.MaintenancePerKm,
				InsuranceCost:   insCost,
				InsuranceRate:   rates.InsurancePerKm,
				TollsCost:       tollsCost,
				TotalCost:       totalCost,
				CostPerKm:       costPerKm,
			},
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"drives": enriched,
		"total":  total,
		"page":   page,
		"limit":  limit,
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
		writeError(w, http.StatusInternalServerError, "Failed to load drive expenses")
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
		writeError(w, http.StatusInternalServerError, "Failed to update tags")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"tags":    req.Tags,
	})
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

	if err := h.repo.CreateTripGroup(r.Context(), tg, req.DriveIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create trip group")
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
