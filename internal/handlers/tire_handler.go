package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type TireHandler struct {
	repo        *database.Repository
	wearService *services.TireWearService
}

func NewTireHandler(repo *database.Repository, wearService *services.TireWearService) *TireHandler {
	return &TireHandler{
		repo:        repo,
		wearService: wearService,
	}
}

func (h *TireHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	tires, err := h.repo.ListTires(r.Context(), vehicleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list tires")
		return
	}

	var statsList []*services.TireWearStats
	for _, tire := range tires {
		stats, err := h.wearService.CalculateTireWear(r.Context(), &tire, v.CurrentOdometer)
		if err == nil {
			statsList = append(statsList, stats)
		}
	}

	if statsList == nil {
		statsList = []*services.TireWearStats{}
	}

	writeJSON(w, http.StatusOK, statsList)
}

type CreateTireRequest struct {
	Brand           string              `json:"brand"`
	Model           string              `json:"model"`
	Dimension       string              `json:"dimension"`
	Season          models.TireSeason   `json:"season"`
	PurchaseDate    string              `json:"purchase_date"` // YYYY-MM-DD
	PurchasePrice   float64             `json:"purchase_price"`
	CurrentPosition models.TirePosition `json:"current_position"`
	InitialDepthMm  float64             `json:"initial_depth_mm"`
	MinLegalDepthMm float64             `json:"min_legal_depth_mm"`
	DotCode         *string             `json:"dot_code"`
}

func (h *TireHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req CreateTireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Brand == "" || req.Model == "" || req.Dimension == "" {
		writeError(w, http.StatusBadRequest, "Brand, model and dimension are required")
		return
	}

	purchaseDate, err := time.Parse("2006-01-02", req.PurchaseDate)
	if err != nil {
		purchaseDate = time.Now()
	}

	pos := req.CurrentPosition
	if pos == "" {
		pos = models.TirePosStorage
	}

	season := req.Season
	if season == "" {
		season = models.TireSeasonSummer
	}

	initialDepth := req.InitialDepthMm
	if initialDepth <= 0 {
		initialDepth = 8.0
	}
	minDepth := req.MinLegalDepthMm
	if minDepth <= 0 {
		minDepth = 1.6
	}

	t := &models.Tire{
		VehicleID:       &vehicleID,
		Brand:           req.Brand,
		Model:           req.Model,
		Dimension:       req.Dimension,
		Season:          season,
		PurchaseDate:    purchaseDate,
		PurchasePrice:   req.PurchasePrice,
		CurrentPosition: pos,
		InitialDepthMm:  initialDepth,
		MinLegalDepthMm: minDepth,
		DotCode:         req.DotCode,
	}

	if err := h.repo.CreateTire(r.Context(), t); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create tire: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

type AddTireLogRequest struct {
	DepthMm  float64 `json:"depth_mm"`
	Odometer float64 `json:"odometer"`
	Notes    *string `json:"notes"`
	Date     string  `json:"date"`
}

func (h *TireHandler) AddLog(w http.ResponseWriter, r *http.Request) {
	tireID := chi.URLParam(r, "tireId")

	var req AddTireLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.DepthMm <= 0 {
		writeError(w, http.StatusBadRequest, "Depth must be greater than 0 mm")
		return
	}

	logDate, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		logDate = time.Now()
	}

	log := &models.TireLog{
		TireID:   tireID,
		Date:     logDate,
		Odometer: req.Odometer,
		DepthMm:  req.DepthMm,
		Notes:    req.Notes,
	}

	if err := h.repo.AddTireLog(r.Context(), log); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to record tire log")
		return
	}

	writeJSON(w, http.StatusCreated, log)
}

type RotationRequest struct {
	Date        string         `json:"date"`
	Odometer    float64        `json:"odometer"`
	MappingJSON map[string]any `json:"mapping_json"` // {"FL": "id1", "FR": "id2", ...}
	Notes       *string        `json:"notes"`
}

func (h *TireHandler) Rotate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req RotationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	rotDate, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		rotDate = time.Now()
	}

	rot := &models.TireRotation{
		VehicleID:   vehicleID,
		Date:        rotDate,
		Odometer:    req.Odometer,
		MappingJSON: req.MappingJSON,
		Notes:       req.Notes,
	}

	if err := h.repo.AddTireRotation(r.Context(), rot); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to apply tire rotation: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, rot)
}
