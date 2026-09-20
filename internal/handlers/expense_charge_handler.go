package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

func (h *ExpenseHandler) ListCharges(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
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
	missingCostOnly := r.URL.Query().Get("missing_cost") == "true"

	charges, total, err := h.repo.ListCharges(r.Context(), vehicleID, missingCostOnly, limit, offset)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list charges")
		return
	}
	if charges == nil {
		charges = []models.ChargeLog{}
	}
	missingCount, err := h.repo.CountChargesWithoutCost(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list charges")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"charges":              charges,
		"total":                total,
		"page":                 page,
		"limit":                limit,
		"charges_without_cost": missingCount,
	})
}

type SaveChargeRequest struct {
	Date       string       `json:"date"`
	EndDate    *string      `json:"end_date"`
	Address    *string      `json:"address"`
	KwhAdded   float64      `json:"kwh_added"`
	Cost       *money.Cents `json:"cost"`
	Currency   string       `json:"currency"`
	FxRate     *float64     `json:"fx_rate"`
	Odometer   *float64     `json:"odometer"`
	Notes      *string      `json:"notes"`
	DocumentID *string      `json:"document_id"`
}

func buildCharge(vehicleID string, req *SaveChargeRequest) (*models.ChargeLog, error) {
	date, err := parseDate(req.Date)
	if err != nil {
		return nil, err
	}
	endDate, err := parseOptionalDate(req.EndDate)
	if err != nil {
		return nil, err
	}
	if endDate != nil && endDate.Before(date) {
		return nil, errors.New("la fin de recharge précède son début")
	}
	if err := validateQuantity(req.KwhAdded, 1000); err != nil {
		return nil, errors.New("énergie ajoutée invalide")
	}
	if req.Cost != nil {
		if err := validateAmount(*req.Cost, true); err != nil {
			return nil, err
		}
	}
	curr, fxRate, err := normalizeCurrency(req.Currency, req.FxRate)
	if err != nil {
		return nil, err
	}
	odometer := req.Odometer
	if odometer != nil && *odometer <= 0 {
		odometer = nil
	}
	return &models.ChargeLog{
		VehicleID:  vehicleID,
		Date:       date,
		EndDate:    endDate,
		Address:    req.Address,
		KwhAdded:   req.KwhAdded,
		Cost:       req.Cost,
		Currency:   curr,
		FxRate:     fxRate,
		Odometer:   odometer,
		Notes:      req.Notes,
		DocumentID: req.DocumentID,
	}, nil
}

// decodeCharge reads and validates a charge from the request body. It writes the error response and returns
// false when the payload is invalid or carries no cost (costRequired is the message for that case).
func decodeCharge(w http.ResponseWriter, r *http.Request, vehicleID, costRequired string) (*models.ChargeLog, bool) {
	var req SaveChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return nil, false
	}
	c, err := buildCharge(vehicleID, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return nil, false
	}
	if c.Cost == nil {
		writeError(w, http.StatusBadRequest, costRequired)
		return nil, false
	}
	return c, true
}

func (h *ExpenseHandler) CreateManualCharge(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	c, ok := decodeCharge(w, r, vehicleID, "le coût d'une recharge manuelle est requis")
	if !ok {
		return
	}
	if c.KwhAdded <= 0 {
		writeError(w, http.StatusBadRequest, "l'énergie ajoutée doit être positive")
		return
	}

	if err := h.repo.CreateManualCharge(r.Context(), c); err != nil {
		writeRepoError(w, r, err, "Failed to record charge")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *ExpenseHandler) UpdateCharge(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	chargeID := chi.URLParam(r, "chargeId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	c, ok := decodeCharge(w, r, vehicleID, "le coût est requis")
	if !ok {
		return
	}
	c.ID = chargeID

	if err := h.repo.UpdateCharge(r.Context(), c); err != nil {
		writeRepoError(w, r, err, "Failed to update charge")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *ExpenseHandler) DeleteManualCharge(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	chargeID := chi.URLParam(r, "chargeId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	if err := h.repo.DeleteManualCharge(r.Context(), vehicleID, chargeID); err != nil {
		writeRepoError(w, r, err, "Failed to delete charge")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// ============================================================================
// Documents & Invoices
// ============================================================================
