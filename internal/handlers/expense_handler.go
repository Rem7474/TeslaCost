package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
)

type ExpenseHandler struct {
	repo *database.Repository
}

func NewExpenseHandler(repo *database.Repository) *ExpenseHandler {
	return &ExpenseHandler{repo: repo}
}

type CreateDriveExpenseRequest struct {
	TripGroupID *string  `json:"trip_group_id"`
	DriveID     *string  `json:"drive_id"`
	DriveIDs    []string `json:"drive_ids"`
	Type        string   `json:"type"` // TOLL, PARKING, etc.
	Amount      float64  `json:"amount"`
	Currency    string   `json:"currency"`
	FxRate      *float64 `json:"fx_rate"`
	Date        string   `json:"date"`
	Notes       *string  `json:"notes"`
}

// buildDriveExpense validates a drive expense payload.
func buildDriveExpense(vehicleID string, req *CreateDriveExpenseRequest) (*models.DriveExpense, error) {
	if err := validateAmount(req.Amount, false); err != nil {
		return nil, err
	}
	expDate, err := parseDate(req.Date)
	if err != nil {
		return nil, err
	}
	curr, fxRate, err := normalizeCurrency(req.Currency, req.FxRate)
	if err != nil {
		return nil, err
	}
	expType := strings.ToUpper(strings.TrimSpace(req.Type))
	if expType == "" {
		expType = "TOLL"
	}
	if !driveExpenseTypes[expType] {
		return nil, errors.New("type de dépense invalide")
	}

	return &models.DriveExpense{
		VehicleID:   vehicleID,
		TripGroupID: req.TripGroupID,
		DriveID:     req.DriveID,
		Type:        expType,
		Amount:      req.Amount,
		Currency:    curr,
		FxRate:      fxRate,
		Date:        expDate,
		Notes:       req.Notes,
	}, nil
}

func expenseGroupName(notes *string) string {
	if notes != nil && strings.TrimSpace(*notes) != "" {
		return *notes
	}
	return "Trajet multi-étapes"
}

func (h *ExpenseHandler) CreateDriveExpense(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req CreateDriveExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	exp, err := buildDriveExpense(vehicleID, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.repo.SaveDriveExpense(r.Context(), exp, req.DriveIDs, expenseGroupName(req.Notes)); err != nil {
		writeRepoError(w, err, "Failed to record expense")
		return
	}

	writeJSON(w, http.StatusCreated, exp)
}

func (h *ExpenseHandler) ListDriveExpenses(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	list, err := h.repo.ListDriveExpenses(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, err, "Failed to list expenses")
		return
	}
	if list == nil {
		list = []models.DriveExpense{}
	}

	writeJSON(w, http.StatusOK, list)
}

func (h *ExpenseHandler) UpdateDriveExpense(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	expenseID := chi.URLParam(r, "expenseId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req CreateDriveExpenseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	exp, err := buildDriveExpense(vehicleID, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	exp.ID = expenseID

	if err := h.repo.SaveDriveExpense(r.Context(), exp, req.DriveIDs, expenseGroupName(req.Notes)); err != nil {
		writeRepoError(w, err, "Failed to update expense")
		return
	}

	writeJSON(w, http.StatusOK, exp)
}

func (h *ExpenseHandler) DeleteDriveExpense(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	expenseID := chi.URLParam(r, "expenseId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	if err := h.repo.DeleteDriveExpense(r.Context(), vehicleID, expenseID); err != nil {
		writeRepoError(w, err, "Failed to delete expense")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Expense deleted successfully"})
}

type CreateMaintenanceRequest struct {
	Category                 string   `json:"category"`
	Amount                   float64  `json:"amount"`
	Currency                 string   `json:"currency"`
	FxRate                   *float64 `json:"fx_rate"`
	Date                     string   `json:"date"`
	Odometer                 *float64 `json:"odometer"`
	IsRecurring              bool     `json:"is_recurring"`
	RecurrenceIntervalMonths *int     `json:"recurrence_interval_months"`
	RecurrenceEndDate        *string  `json:"recurrence_end_date"`
	Description              string   `json:"description"`
}

// buildMaintenanceExpense validates a maintenance / fixed expense payload.
func buildMaintenanceExpense(vehicleID string, req *CreateMaintenanceRequest) (*models.MaintenanceExpense, error) {
	category := strings.ToUpper(strings.TrimSpace(req.Category))
	if !maintenanceCategories[category] {
		return nil, errors.New("catégorie de dépense invalide")
	}
	if err := validateAmount(req.Amount, false); err != nil {
		return nil, err
	}
	mDate, err := parseDate(req.Date)
	if err != nil {
		return nil, err
	}
	curr, fxRate, err := normalizeCurrency(req.Currency, req.FxRate)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Description) == "" {
		return nil, errors.New("la description est requise")
	}

	interval := req.RecurrenceIntervalMonths
	var endDate *time.Time
	if req.IsRecurring {
		if interval == nil || *interval <= 0 || *interval > 120 {
			return nil, errors.New("une dépense récurrente requiert une périodicité en mois (1 à 120)")
		}
		if endDate, err = parseOptionalDate(req.RecurrenceEndDate); err != nil {
			return nil, err
		}
		if endDate != nil && endDate.Before(mDate) {
			return nil, errors.New("la fin de récurrence précède la première échéance")
		}
	} else {
		interval = nil
	}

	odometer := req.Odometer
	if odometer != nil && *odometer <= 0 {
		odometer = nil
	}

	return &models.MaintenanceExpense{
		VehicleID:                vehicleID,
		Category:                 category,
		Amount:                   req.Amount,
		Currency:                 curr,
		FxRate:                   fxRate,
		Date:                     mDate,
		Odometer:                 odometer,
		IsRecurring:              req.IsRecurring,
		RecurrenceIntervalMonths: interval,
		RecurrenceEndDate:        endDate,
		Description:              req.Description,
	}, nil
}

func (h *ExpenseHandler) CreateMaintenance(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req CreateMaintenanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	m, err := buildMaintenanceExpense(vehicleID, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.repo.CreateMaintenanceExpense(r.Context(), m); err != nil {
		writeRepoError(w, err, "Failed to record maintenance expense")
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

func (h *ExpenseHandler) ListMaintenance(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	list, err := h.repo.ListMaintenanceExpenses(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, err, "Failed to list maintenance expenses")
		return
	}
	if list == nil {
		list = []models.MaintenanceExpense{}
	}

	writeJSON(w, http.StatusOK, list)
}

func (h *ExpenseHandler) UpdateMaintenance(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	maintID := chi.URLParam(r, "maintenanceId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req CreateMaintenanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	m, err := buildMaintenanceExpense(vehicleID, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	m.ID = maintID

	if err := h.repo.UpdateMaintenanceExpense(r.Context(), m); err != nil {
		writeRepoError(w, err, "Failed to update maintenance expense")
		return
	}

	writeJSON(w, http.StatusOK, m)
}

func (h *ExpenseHandler) DeleteMaintenance(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	maintID := chi.URLParam(r, "maintenanceId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	if err := h.repo.DeleteMaintenanceExpense(r.Context(), vehicleID, maintID); err != nil {
		writeRepoError(w, err, "Failed to delete maintenance expense")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Maintenance expense deleted successfully"})
}

func (h *ExpenseHandler) ListCharges(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
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
		writeRepoError(w, err, "Failed to list charges")
		return
	}
	if charges == nil {
		charges = []models.ChargeLog{}
	}
	missingCount, err := h.repo.CountChargesWithoutCost(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, err, "Failed to list charges")
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
	Date     string   `json:"date"`
	EndDate  *string  `json:"end_date"`
	Address  *string  `json:"address"`
	KwhAdded float64  `json:"kwh_added"`
	Cost     *float64 `json:"cost"`
	Currency string   `json:"currency"`
	FxRate   *float64 `json:"fx_rate"`
	Odometer *float64 `json:"odometer"`
	Notes    *string  `json:"notes"`
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
	if err := validateAmount(req.KwhAdded, true); err != nil || req.KwhAdded > 1000 {
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
		VehicleID: vehicleID,
		Date:      date,
		EndDate:   endDate,
		Address:   req.Address,
		KwhAdded:  req.KwhAdded,
		Cost:      req.Cost,
		Currency:  curr,
		FxRate:    fxRate,
		Odometer:  odometer,
		Notes:     req.Notes,
	}, nil
}

func (h *ExpenseHandler) CreateManualCharge(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req SaveChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	c, err := buildCharge(vehicleID, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if c.Cost == nil {
		writeError(w, http.StatusBadRequest, "le coût d'une recharge manuelle est requis")
		return
	}
	if c.KwhAdded <= 0 {
		writeError(w, http.StatusBadRequest, "l'énergie ajoutée doit être positive")
		return
	}

	if err := h.repo.CreateManualCharge(r.Context(), c); err != nil {
		writeRepoError(w, err, "Failed to record charge")
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *ExpenseHandler) UpdateCharge(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	chargeID := chi.URLParam(r, "chargeId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	var req SaveChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	c, err := buildCharge(vehicleID, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if c.Cost == nil {
		writeError(w, http.StatusBadRequest, "le coût est requis")
		return
	}
	c.ID = chargeID

	if err := h.repo.UpdateCharge(r.Context(), c); err != nil {
		writeRepoError(w, err, "Failed to update charge")
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (h *ExpenseHandler) DeleteManualCharge(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "vehicleId")
	chargeID := chi.URLParam(r, "chargeId")

	if _, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID); err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}

	if err := h.repo.DeleteManualCharge(r.Context(), vehicleID, chargeID); err != nil {
		writeRepoError(w, err, "Failed to delete charge")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
