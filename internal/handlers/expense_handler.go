package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
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
	Date        string   `json:"date"`
	Notes       *string  `json:"notes"`
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

	expDate, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		expDate = time.Now()
	}

	curr := req.Currency
	if curr == "" {
		curr = "EUR"
	}

	expType := req.Type
	if expType == "" {
		expType = "TOLL"
	}

	var driveID = req.DriveID
	var tripGroupID = req.TripGroupID

	if len(req.DriveIDs) > 1 {
		groupName := "Trajet multi-étapes"
		if req.Notes != nil && *req.Notes != "" {
			groupName = *req.Notes
		}
		tg := &models.TripGroup{
			VehicleID: vehicleID,
			Name:      groupName,
			Notes:     req.Notes,
		}
		if err := h.repo.CreateTripGroup(r.Context(), tg, req.DriveIDs); err == nil {
			tripGroupID = &tg.ID
		}
	} else if len(req.DriveIDs) == 1 {
		driveID = &req.DriveIDs[0]
	}

	exp := &models.DriveExpense{
		VehicleID:   vehicleID,
		TripGroupID: tripGroupID,
		DriveID:     driveID,
		Type:        expType,
		Amount:      req.Amount,
		Currency:    curr,
		Date:        expDate,
		Notes:       req.Notes,
	}

	if err := h.repo.CreateDriveExpense(r.Context(), exp); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to record expense")
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
		writeError(w, http.StatusInternalServerError, "Failed to list expenses")
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

	expDate, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		expDate = time.Now()
	}

	curr := req.Currency
	if curr == "" {
		curr = "EUR"
	}

	expType := req.Type
	if expType == "" {
		expType = "TOLL"
	}

	var driveID = req.DriveID
	var tripGroupID = req.TripGroupID

	if len(req.DriveIDs) > 1 {
		groupName := "Trajet multi-étapes"
		if req.Notes != nil && *req.Notes != "" {
			groupName = *req.Notes
		}
		tg := &models.TripGroup{
			VehicleID: vehicleID,
			Name:      groupName,
			Notes:     req.Notes,
		}
		if err := h.repo.CreateTripGroup(r.Context(), tg, req.DriveIDs); err == nil {
			tripGroupID = &tg.ID
		}
	} else if len(req.DriveIDs) == 1 {
		driveID = &req.DriveIDs[0]
	}

	exp := &models.DriveExpense{
		ID:          expenseID,
		VehicleID:   vehicleID,
		TripGroupID: tripGroupID,
		DriveID:     driveID,
		Type:        expType,
		Amount:      req.Amount,
		Currency:    curr,
		Date:        expDate,
		Notes:       req.Notes,
	}

	if err := h.repo.UpdateDriveExpense(r.Context(), exp); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update expense")
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
		writeError(w, http.StatusInternalServerError, "Failed to delete expense")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Expense deleted successfully"})
}

type CreateMaintenanceRequest struct {
	Category                 string   `json:"category"`
	Amount                   float64  `json:"amount"`
	Currency                 string   `json:"currency"`
	Date                     string   `json:"date"`
	Odometer                 *float64 `json:"odometer"`
	IsRecurring              bool     `json:"is_recurring"`
	RecurrenceIntervalMonths *int     `json:"recurrence_interval_months"`
	Description              string   `json:"description"`
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

	mDate, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		mDate = time.Now()
	}

	curr := req.Currency
	if curr == "" {
		curr = "EUR"
	}

	m := &models.MaintenanceExpense{
		VehicleID:                vehicleID,
		Category:                 req.Category,
		Amount:                   req.Amount,
		Currency:                 curr,
		Date:                     mDate,
		Odometer:                 req.Odometer,
		IsRecurring:              req.IsRecurring,
		RecurrenceIntervalMonths: req.RecurrenceIntervalMonths,
		Description:              req.Description,
	}

	if err := h.repo.CreateMaintenanceExpense(r.Context(), m); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to record maintenance expense")
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
		writeError(w, http.StatusInternalServerError, "Failed to list maintenance expenses")
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

	mDate, err := time.Parse(time.RFC3339, req.Date)
	if err != nil {
		mDate = time.Now()
	}

	curr := req.Currency
	if curr == "" {
		curr = "EUR"
	}

	m := &models.MaintenanceExpense{
		ID:                       maintID,
		VehicleID:                vehicleID,
		Category:                 req.Category,
		Amount:                   req.Amount,
		Currency:                 curr,
		Date:                     mDate,
		Odometer:                 req.Odometer,
		IsRecurring:              req.IsRecurring,
		RecurrenceIntervalMonths: req.RecurrenceIntervalMonths,
		Description:              req.Description,
	}

	if err := h.repo.UpdateMaintenanceExpense(r.Context(), m); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update maintenance expense")
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
		writeError(w, http.StatusInternalServerError, "Failed to delete maintenance expense")
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

	charges, total, err := h.repo.ListCharges(r.Context(), vehicleID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list charges")
		return
	}
	if charges == nil {
		charges = []models.ChargeLog{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"charges": charges,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}
