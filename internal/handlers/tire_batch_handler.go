package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

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
	at, ok := parseDisposal(w, req.Date, req.Odometer)
	if !ok {
		return
	}
	if err := h.repo.BatchDisposeTires(r.Context(), vehicleID, req.TireIDs, at, req.Odometer); err != nil {
		writeRepoError(w, r, err, "Failed to batch dispose tires")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
