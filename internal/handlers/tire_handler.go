package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/services"
)

type TireHandler struct {
	repo            *database.Repository
	tireWearService *services.TireWearService
}

func NewTireHandler(repo *database.Repository, tireWearService *services.TireWearService) *TireHandler {
	return &TireHandler{
		repo:            repo,
		tireWearService: tireWearService,
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

	var statsList []services.TireWearStats
	for i := range tires {
		stats, err := h.tireWearService.CalculateTireWear(r.Context(), &tires[i], v.CurrentOdometer)
		if err == nil && stats != nil {
			statsList = append(statsList, *stats)
		}
	}

	writeJSON(w, http.StatusOK, statsList)
}

type CreateTireRequest struct {
	Brand                 string              `json:"brand"`
	Model                 string              `json:"model"`
	Dimension             string              `json:"dimension"`
	Season                models.TireSeason   `json:"season"`
	PurchaseDate          string              `json:"purchase_date"` // YYYY-MM-DD
	PurchasePrice         float64             `json:"purchase_price"`
	CurrentPosition       models.TirePosition `json:"current_position"`
	InitialDepthMm        float64             `json:"initial_depth_mm"`
	MinLegalDepthMm       float64             `json:"min_legal_depth_mm"`
	DotCode               *string             `json:"dot_code"`
	MountedOdometer       *float64            `json:"mounted_odometer"`
	AccumulatedDistanceKm float64             `json:"accumulated_distance_km"`
	EstimatedLifespanKm   int                 `json:"estimated_lifespan_km"`
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

	lifespan := req.EstimatedLifespanKm
	if lifespan <= 0 {
		lifespan = 40000
	}

	t := &models.Tire{
		VehicleID:             &vehicleID,
		Brand:                 req.Brand,
		Model:                 req.Model,
		Dimension:             req.Dimension,
		Season:                season,
		PurchaseDate:          purchaseDate,
		PurchasePrice:         req.PurchasePrice,
		CurrentPosition:       pos,
		InitialDepthMm:        initialDepth,
		MinLegalDepthMm:       minDepth,
		DotCode:               req.DotCode,
		MountedOdometer:       req.MountedOdometer,
		AccumulatedDistanceKm: req.AccumulatedDistanceKm,
		EstimatedLifespanKm:   lifespan,
	}

	if err := h.repo.CreateTire(r.Context(), t); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create tire: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

type BatchCreateTiresRequest struct {
	Type                  string            `json:"type"` // "SET_4", "SET_2_FRONT", "SET_2_REAR", "SET_4_STORAGE", "SET_2_STORAGE"
	Brand                 string            `json:"brand"`
	Model                 string            `json:"model"`
	Dimension             string            `json:"dimension"`
	Season                models.TireSeason `json:"season"`
	PurchaseDate          string            `json:"purchase_date"`
	TotalPrice            float64           `json:"total_price"`
	UnitPrice             float64           `json:"unit_price"`
	InitialDepthMm        float64           `json:"initial_depth_mm"`
	MinLegalDepthMm       float64           `json:"min_legal_depth_mm"`
	DotCode               *string           `json:"dot_code"`
	MountedOdometer       *float64          `json:"mounted_odometer"`
	AccumulatedDistanceKm float64           `json:"accumulated_distance_km"`
	EstimatedLifespanKm   int               `json:"estimated_lifespan_km"`
}

func (h *TireHandler) BatchCreate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req BatchCreateTiresRequest
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

	lifespan := req.EstimatedLifespanKm
	if lifespan <= 0 {
		lifespan = 40000
	}

	var positions []models.TirePosition
	switch req.Type {
	case "SET_2_FRONT":
		positions = []models.TirePosition{models.TirePosFL, models.TirePosFR}
	case "SET_2_REAR":
		positions = []models.TirePosition{models.TirePosRL, models.TirePosRR}
	case "SET_4_STORAGE":
		positions = []models.TirePosition{models.TirePosStorage, models.TirePosStorage, models.TirePosStorage, models.TirePosStorage}
	case "SET_2_STORAGE":
		positions = []models.TirePosition{models.TirePosStorage, models.TirePosStorage}
	case "SET_4":
		fallthrough
	default:
		positions = []models.TirePosition{models.TirePosFL, models.TirePosFR, models.TirePosRL, models.TirePosRR}
	}

	count := len(positions)
	unitPrice := req.UnitPrice
	if req.TotalPrice > 0 && count > 0 {
		unitPrice = math.Round((req.TotalPrice/float64(count))*100) / 100
	}

	var tires []*models.Tire
	for _, pos := range positions {
		var mountedOdom *float64
		if pos != models.TirePosStorage {
			mountedOdom = req.MountedOdometer
		}
		t := &models.Tire{
			VehicleID:             &vehicleID,
			Brand:                 req.Brand,
			Model:                 req.Model,
			Dimension:             req.Dimension,
			Season:                season,
			PurchaseDate:          purchaseDate,
			PurchasePrice:         unitPrice,
			CurrentPosition:       pos,
			InitialDepthMm:        initialDepth,
			MinLegalDepthMm:       minDepth,
			DotCode:               req.DotCode,
			MountedOdometer:       mountedOdom,
			AccumulatedDistanceKm: req.AccumulatedDistanceKm,
			EstimatedLifespanKm:   lifespan,
		}
		tires = append(tires, t)
	}

	if err := h.repo.CreateTiresBatch(r.Context(), tires); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to batch create tires: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"success": true,
		"count":   count,
		"tires":   tires,
	})
}

type QuickRotateRequest struct {
	Mode                string   `json:"mode"` // "FRONT_BACK", "CROSS", "SWAP_PACK"
	Odometer            float64  `json:"odometer"`
	SwapWithPackTireIDs []string `json:"swap_with_pack_tire_ids"`
}

func (h *TireHandler) QuickRotate(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req QuickRotateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.repo.QuickRotateTires(r.Context(), vehicleID, req.Mode, req.Odometer, req.SwapWithPackTireIDs); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to perform quick rotation: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (h *TireHandler) Update(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	t, err := h.repo.GetTireByID(r.Context(), tireID, vehicleID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Tire not found")
		return
	}

	var req CreateTireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Brand != "" {
		t.Brand = req.Brand
	}
	if req.Model != "" {
		t.Model = req.Model
	}
	if req.Dimension != "" {
		t.Dimension = req.Dimension
	}
	if req.Season != "" {
		t.Season = req.Season
	}
	if req.PurchaseDate != "" {
		if pd, err := time.Parse("2006-01-02", req.PurchaseDate); err == nil {
			t.PurchaseDate = pd
		}
	}
	if req.PurchasePrice > 0 {
		t.PurchasePrice = req.PurchasePrice
	}
	if req.CurrentPosition != "" {
		t.CurrentPosition = req.CurrentPosition
	}
	if req.InitialDepthMm > 0 {
		t.InitialDepthMm = req.InitialDepthMm
	}
	if req.MinLegalDepthMm > 0 {
		t.MinLegalDepthMm = req.MinLegalDepthMm
	}
	if req.DotCode != nil {
		t.DotCode = req.DotCode
	}
	if req.MountedOdometer != nil {
		t.MountedOdometer = req.MountedOdometer
	}
	t.AccumulatedDistanceKm = req.AccumulatedDistanceKm
	if req.EstimatedLifespanKm > 0 {
		t.EstimatedLifespanKm = req.EstimatedLifespanKm
	}

	if err := h.repo.UpdateTire(r.Context(), t); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update tire: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, t)
}

func (h *TireHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")

	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	t, err := h.repo.GetTireByID(r.Context(), tireID, vehicleID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Tire not found")
		return
	}

	stats, _ := h.tireWearService.CalculateTireWear(r.Context(), t, v.CurrentOdometer)
	sessions, _ := h.repo.ListTireMountSessions(r.Context(), tireID)
	logs, _ := h.repo.ListTireLogs(r.Context(), tireID)

	writeJSON(w, http.StatusOK, map[string]any{
		"tire":     t,
		"stats":    stats,
		"sessions": sessions,
		"logs":     logs,
	})
}

type MountSessionPayload struct {
	Position           models.TirePosition `json:"position"`
	MountedDate        string              `json:"mounted_date"`
	MountedOdometer    float64             `json:"mounted_odometer"`
	DismountedDate     *string             `json:"dismounted_date"`
	DismountedOdometer *float64            `json:"dismounted_odometer"`
	DistanceKm         float64             `json:"distance_km"`
	Notes              *string             `json:"notes"`
}

func (h *TireHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req MountSessionPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	mountedDate, err := time.Parse(time.RFC3339, req.MountedDate)
	if err != nil {
		mountedDate = time.Now()
	}

	var dismountedDate *time.Time
	if req.DismountedDate != nil && *req.DismountedDate != "" {
		if dd, err := time.Parse(time.RFC3339, *req.DismountedDate); err == nil {
			dismountedDate = &dd
		}
	}

	session := &models.TireMountSession{
		TireID:             tireID,
		VehicleID:          vehicleID,
		Position:           req.Position,
		MountedDate:        mountedDate,
		MountedOdometer:    req.MountedOdometer,
		DismountedDate:     dismountedDate,
		DismountedOdometer: req.DismountedOdometer,
		DistanceKm:         req.DistanceKm,
		Notes:              req.Notes,
	}

	if err := h.repo.CreateTireMountSession(r.Context(), session); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create mount session: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, session)
}

func (h *TireHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")
	sessionID := chi.URLParam(r, "sessionId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req MountSessionPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	mountedDate, err := time.Parse(time.RFC3339, req.MountedDate)
	if err != nil {
		mountedDate = time.Now()
	}

	var dismountedDate *time.Time
	if req.DismountedDate != nil && *req.DismountedDate != "" {
		if dd, err := time.Parse(time.RFC3339, *req.DismountedDate); err == nil {
			dismountedDate = &dd
		}
	}

	session := &models.TireMountSession{
		ID:                 sessionID,
		TireID:             tireID,
		VehicleID:          vehicleID,
		Position:           req.Position,
		MountedDate:        mountedDate,
		MountedOdometer:    req.MountedOdometer,
		DismountedDate:     dismountedDate,
		DismountedOdometer: req.DismountedOdometer,
		DistanceKm:         req.DistanceKm,
		Notes:              req.Notes,
	}

	if err := h.repo.UpdateTireMountSession(r.Context(), session); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update mount session: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, session)
}

func (h *TireHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")
	sessionID := chi.URLParam(r, "sessionId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	if err := h.repo.DeleteTireMountSession(r.Context(), sessionID, tireID); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete mount session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
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
	MappingJSON map[string]any `json:"mapping_json"`
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
