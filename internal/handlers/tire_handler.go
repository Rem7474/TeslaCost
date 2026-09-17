package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
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
	vehicleID := chi.URLParam(r, "vehicleId")

	v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer)
	if v == nil {
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
	PurchasePrice         money.Cents         `json:"purchase_price"`
	CurrentPosition       models.TirePosition `json:"current_position"`
	InitialDepthMm        float64             `json:"initial_depth_mm"`
	MinLegalDepthMm       float64             `json:"min_legal_depth_mm"`
	DotCode               *string             `json:"dot_code"`
	MountedOdometer       *float64            `json:"mounted_odometer"`
	AccumulatedDistanceKm float64             `json:"accumulated_distance_km"`
	EstimatedLifespanKm   int                 `json:"estimated_lifespan_km"`
}

func (h *TireHandler) Create(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
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

	purchaseDate, err := parseDate(req.PurchaseDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateAmount(req.PurchasePrice, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
		writeRepoError(w, r, err, "Failed to create tire")
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
	TotalPrice            money.Cents       `json:"total_price"`
	UnitPrice             money.Cents       `json:"unit_price"`
	InitialDepthMm        float64           `json:"initial_depth_mm"`
	MinLegalDepthMm       float64           `json:"min_legal_depth_mm"`
	DotCode               *string           `json:"dot_code"`
	MountedOdometer       *float64          `json:"mounted_odometer"`
	AccumulatedDistanceKm float64           `json:"accumulated_distance_km"`
	EstimatedLifespanKm   int               `json:"estimated_lifespan_km"`
}

func (h *TireHandler) BatchCreate(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
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

	purchaseDate, err := parseDate(req.PurchaseDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateAmount(req.TotalPrice, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateAmount(req.UnitPrice, true); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
	prices := make([]money.Cents, count)
	for i := range prices {
		prices[i] = req.UnitPrice
	}
	if req.TotalPrice > 0 {
		prices = money.Split(req.TotalPrice, count)
	}

	var tires []*models.Tire
	for i, pos := range positions {
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
			PurchasePrice:         prices[i],
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
		writeRepoError(w, r, err, "Failed to batch create tires")
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
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req QuickRotateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if err := h.repo.QuickRotateTires(r.Context(), vehicleID, req.Mode, req.Odometer, req.SwapWithPackTireIDs); err != nil {
		writeRepoError(w, r, err, "Failed to perform quick rotation")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// UpdateTireRequest updates descriptive fields; omitted fields are left unchanged.
// Positions are changed through mount sessions or rotations only.
type UpdateTireRequest struct {
	Brand               *string            `json:"brand"`
	Model               *string            `json:"model"`
	Dimension           *string            `json:"dimension"`
	Season              *models.TireSeason `json:"season"`
	PurchaseDate        *string            `json:"purchase_date"`
	PurchasePrice       *money.Cents       `json:"purchase_price"`
	InitialDepthMm      *float64           `json:"initial_depth_mm"`
	MinLegalDepthMm     *float64           `json:"min_legal_depth_mm"`
	DotCode             *string            `json:"dot_code"`
	InitialDistanceKm   *float64           `json:"initial_distance_km"`
	EstimatedLifespanKm *int               `json:"estimated_lifespan_km"`
}

func (h *TireHandler) Update(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	t, err := h.repo.GetTireByID(r.Context(), tireID, vehicleID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Tire not found")
		return
	}

	var req UpdateTireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.Brand != nil && *req.Brand != "" {
		t.Brand = *req.Brand
	}
	if req.Model != nil && *req.Model != "" {
		t.Model = *req.Model
	}
	if req.Dimension != nil && *req.Dimension != "" {
		t.Dimension = *req.Dimension
	}
	if req.Season != nil && *req.Season != "" {
		t.Season = *req.Season
	}
	if req.PurchaseDate != nil {
		pd, err := parseDate(*req.PurchaseDate)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		t.PurchaseDate = pd
	}
	if req.PurchasePrice != nil {
		if err := validateAmount(*req.PurchasePrice, true); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		t.PurchasePrice = *req.PurchasePrice
	}
	if req.InitialDepthMm != nil && *req.InitialDepthMm > 0 {
		t.InitialDepthMm = *req.InitialDepthMm
	}
	if req.MinLegalDepthMm != nil && *req.MinLegalDepthMm > 0 {
		t.MinLegalDepthMm = *req.MinLegalDepthMm
	}
	if req.DotCode != nil {
		t.DotCode = req.DotCode
	}
	if req.InitialDistanceKm != nil {
		if *req.InitialDistanceKm < 0 {
			writeError(w, http.StatusBadRequest, "le kilométrage initial ne peut pas être négatif")
			return
		}
		t.InitialDistanceKm = *req.InitialDistanceKm
	}
	if req.EstimatedLifespanKm != nil && *req.EstimatedLifespanKm > 0 {
		t.EstimatedLifespanKm = *req.EstimatedLifespanKm
	}

	if err := h.repo.UpdateTire(r.Context(), t); err != nil {
		writeRepoError(w, r, err, "Failed to update tire")
		return
	}

	writeJSON(w, http.StatusOK, t)
}

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

type MountSessionPayload struct {
	Position           models.TirePosition `json:"position"`
	MountedDate        string              `json:"mounted_date"`
	MountedOdometer    float64             `json:"mounted_odometer"`
	DismountedDate     *string             `json:"dismounted_date"`
	DismountedOdometer *float64            `json:"dismounted_odometer"`
	DistanceKm         float64             `json:"distance_km"`
	Notes              *string             `json:"notes"`
}

func parseSessionPayload(req *MountSessionPayload) (time.Time, *time.Time, error) {
	// Historical (completed) sessions may be stored without a precise wheel position.
	// In that case the frontend sends position=STORAGE, which is accepted only when
	// a dismounted_date is provided (i.e. the session is already over).
	isHistoricalStorage := req.Position == models.TirePosStorage && req.DismountedDate != nil
	if !isMountedPosition(req.Position) && !isHistoricalStorage {
		return time.Time{}, nil, errors.New("une session de montage requiert une position FL, FR, RL ou RR (ou STORAGE pour une session historique terminée)")
	}
	if req.MountedOdometer < 0 || (req.DismountedOdometer != nil && *req.DismountedOdometer < 0) {
		return time.Time{}, nil, errors.New("odomètre invalide")
	}
	mountedDate, err := parseDate(req.MountedDate)
	if err != nil {
		return time.Time{}, nil, err
	}
	dismountedDate, err := parseOptionalDate(req.DismountedDate)
	if err != nil {
		return time.Time{}, nil, err
	}
	if dismountedDate != nil && dismountedDate.Before(mountedDate) {
		return time.Time{}, nil, errors.New("la date de démontage précède la date de montage")
	}
	return mountedDate, dismountedDate, nil
}

func isMountedPosition(pos models.TirePosition) bool {
	switch pos {
	case models.TirePosFL, models.TirePosFR, models.TirePosRL, models.TirePosRR:
		return true
	}
	return false
}

func (h *TireHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req MountSessionPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	mountedDate, dismountedDate, err := parseSessionPayload(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
		writeRepoError(w, r, err, "Failed to create mount session")
		return
	}

	writeJSON(w, http.StatusCreated, session)
}

func (h *TireHandler) UpdateSession(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")
	sessionID := chi.URLParam(r, "sessionId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req MountSessionPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	mountedDate, dismountedDate, err := parseSessionPayload(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
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
		writeRepoError(w, r, err, "Failed to update mount session")
		return
	}

	writeJSON(w, http.StatusOK, session)
}

func (h *TireHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")
	sessionID := chi.URLParam(r, "sessionId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	if err := h.repo.DeleteTireMountSession(r.Context(), vehicleID, sessionID, tireID); err != nil {
		writeRepoError(w, r, err, "Failed to delete mount session")
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
	vehicleID := chi.URLParam(r, "vehicleId")
	tireID := chi.URLParam(r, "tireId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	if err := h.repo.EnsureTireOwned(r.Context(), vehicleID, tireID); err != nil {
		writeRepoError(w, r, err, "Failed to record tire log")
		return
	}

	var req AddTireLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	if req.DepthMm <= 0 || req.DepthMm > 20 {
		writeError(w, http.StatusBadRequest, "La profondeur doit être comprise entre 0 et 20 mm")
		return
	}
	if req.Odometer <= 0 {
		writeError(w, http.StatusBadRequest, "Le relevé d'usure requiert l'odomètre du véhicule")
		return
	}

	logDate := time.Now()
	if req.Date != "" {
		parsed, err := parseDate(req.Date)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		logDate = parsed
	}

	log := &models.TireLog{
		TireID:   tireID,
		Date:     logDate,
		Odometer: req.Odometer,
		DepthMm:  req.DepthMm,
		Notes:    req.Notes,
	}

	if err := h.repo.AddTireLog(r.Context(), log); err != nil {
		writeRepoError(w, r, err, "Failed to record tire log")
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
	vehicleID := chi.URLParam(r, "vehicleId")

	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req RotationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	rotDate := time.Now().UTC()
	if req.Date != "" {
		parsed, err := parseDate(req.Date)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		rotDate = parsed
	}

	rot := &models.TireRotation{
		VehicleID:   vehicleID,
		Date:        rotDate,
		Odometer:    req.Odometer,
		MappingJSON: req.MappingJSON,
		Notes:       req.Notes,
	}

	if err := h.repo.AddTireRotation(r.Context(), rot); err != nil {
		writeRepoError(w, r, err, "Failed to apply tire rotation")
		return
	}

	writeJSON(w, http.StatusCreated, rot)
}

// BatchUpdateTiresRequest applies the same values to several tires; omitted fields are left unchanged.
type BatchUpdateTiresRequest struct {
	TireIDs             []string           `json:"tire_ids"`
	Brand               *string            `json:"brand"`
	Model               *string            `json:"model"`
	Dimension           *string            `json:"dimension"`
	Season              *models.TireSeason `json:"season"`
	PurchaseDate        *string            `json:"purchase_date"`
	PurchasePrice       *money.Cents       `json:"purchase_price"` // Unit price
	TotalPrice          *money.Cents       `json:"total_price"`    // Split across the selected tires
	InitialDepthMm      *float64           `json:"initial_depth_mm"`
	MinLegalDepthMm     *float64           `json:"min_legal_depth_mm"`
	DotCode             *string            `json:"dot_code"`
	InitialDistanceKm   *float64           `json:"initial_distance_km"`
	EstimatedLifespanKm *int               `json:"estimated_lifespan_km"`
	MountedDate         *string            `json:"mounted_date"`
	MountedOdometer     *float64           `json:"mounted_odometer"`
}

func nonEmpty(s *string) *string {
	if s == nil || strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}

func buildTirePatch(req *BatchUpdateTiresRequest) (database.TirePatch, error) {
	p := database.TirePatch{
		Brand: nonEmpty(req.Brand), Model: nonEmpty(req.Model), Dimension: nonEmpty(req.Dimension),
		DotCode: req.DotCode, EstimatedLifespanKm: req.EstimatedLifespanKm,
	}
	if req.Season != nil && *req.Season != "" {
		switch *req.Season {
		case models.TireSeasonSummer, models.TireSeasonWinter, models.TireSeasonAllSeason:
			p.Season = req.Season
		default:
			return p, errors.New("saison invalide")
		}
	}
	var err error
	if p.PurchaseDate, err = parseOptionalDate(req.PurchaseDate); err != nil {
		return p, err
	}
	if p.MountedDate, err = parseOptionalDate(req.MountedDate); err != nil {
		return p, err
	}
	for _, amount := range []*money.Cents{req.PurchasePrice, req.TotalPrice} {
		if amount != nil {
			if err := validateAmount(*amount, true); err != nil {
				return p, err
			}
		}
	}
	p.PurchasePrice, p.TotalPrice = req.PurchasePrice, req.TotalPrice
	for _, q := range []struct {
		v   *float64
		max float64
		msg string
	}{
		{req.InitialDepthMm, 20, "profondeur initiale invalide"},
		{req.MinLegalDepthMm, 20, "profondeur minimale invalide"},
		{req.InitialDistanceKm, 500_000, "kilométrage initial invalide"},
		{req.MountedOdometer, 2_000_000, "odomètre de montage invalide"},
	} {
		if q.v != nil {
			if err := validateQuantity(*q.v, q.max); err != nil {
				return p, errors.New(q.msg)
			}
		}
	}
	if req.EstimatedLifespanKm != nil && (*req.EstimatedLifespanKm <= 0 || *req.EstimatedLifespanKm > 500_000) {
		return p, errors.New("durée de vie estimée invalide")
	}
	p.InitialDepthMm, p.MinLegalDepthMm, p.InitialDistanceKm, p.MountedOdometer =
		req.InitialDepthMm, req.MinLegalDepthMm, req.InitialDistanceKm, req.MountedOdometer
	return p, nil
}

// BatchUpdate edits several tires at once (e.g. a set of four bought and mounted together).
func (h *TireHandler) BatchUpdate(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	var req BatchUpdateTiresRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	patch, err := buildTirePatch(&req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.repo.BatchUpdateTires(r.Context(), vehicleID, req.TireIDs, patch); err != nil {
		writeRepoError(w, r, err, "Failed to update tires")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "count": len(req.TireIDs)})
}

// Delete permanently removes a tire entered by mistake, with its history.
func (h *TireHandler) Delete(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	if err := h.repo.DeleteTire(r.Context(), vehicleID, chi.URLParam(r, "tireId")); err != nil {
		writeRepoError(w, r, err, "Failed to delete tire")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

type DisposeTireRequest struct {
	Date     string   `json:"date"`
	Odometer *float64 `json:"odometer"`
}

// Dispose retires a worn out or damaged tire while keeping its history and cost.
func (h *TireHandler) Dispose(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	var req DisposeTireRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	at := time.Now().UTC()
	if req.Date != "" {
		parsed, err := parseDate(req.Date)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		at = parsed
	}
	if req.Odometer != nil {
		if err := validateQuantity(*req.Odometer, 2_000_000); err != nil {
			writeError(w, http.StatusBadRequest, "odomètre invalide")
			return
		}
	}
	if err := h.repo.DisposeTire(r.Context(), vehicleID, chi.URLParam(r, "tireId"), at, req.Odometer); err != nil {
		writeRepoError(w, r, err, "Failed to dispose tire")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// UpdateLog corrects a tread depth measurement.
func (h *TireHandler) UpdateLog(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	var req AddTireLogRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	if req.DepthMm <= 0 || req.DepthMm > 20 || req.Odometer <= 0 {
		writeError(w, http.StatusBadRequest, "Profondeur (0 à 20 mm) et odomètre requis")
		return
	}
	date, err := parseDate(req.Date)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	l := &models.TireLog{ID: chi.URLParam(r, "logId"), TireID: chi.URLParam(r, "tireId"), Date: date, Odometer: req.Odometer, DepthMm: req.DepthMm, Notes: req.Notes}
	if err := h.repo.UpdateTireLog(r.Context(), vehicleID, l); err != nil {
		writeRepoError(w, r, err, "Failed to update tire log")
		return
	}
	writeJSON(w, http.StatusOK, l)
}

// DeleteLog deletes a tread depth measurement.
func (h *TireHandler) DeleteLog(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	if err := h.repo.DeleteTireLog(r.Context(), vehicleID, chi.URLParam(r, "tireId"), chi.URLParam(r, "logId")); err != nil {
		writeRepoError(w, r, err, "Failed to delete tire log")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

type BatchDisposeTiresRequest struct {
	TireIDs  []string `json:"tire_ids"`
	Date     string   `json:"date"`
	Odometer *float64 `json:"odometer"`
}

func (h *TireHandler) BatchDispose(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}
	var req BatchDisposeTiresRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	if len(req.TireIDs) == 0 {
		writeError(w, http.StatusBadRequest, "tire_ids requis")
		return
	}
	at := time.Now().UTC()
	if req.Date != "" {
		parsed, err := parseDate(req.Date)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		at = parsed
	}
	if req.Odometer != nil {
		if err := validateQuantity(*req.Odometer, 2_000_000); err != nil {
			writeError(w, http.StatusBadRequest, "odomètre invalide")
			return
		}
	}
	if err := h.repo.BatchDisposeTires(r.Context(), vehicleID, req.TireIDs, at, req.Odometer); err != nil {
		writeRepoError(w, r, err, "Failed to batch dispose tires")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
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
