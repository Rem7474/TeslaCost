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
	TripGroupID *string `json:"trip_group_id"`
	DriveID     *string `json:"drive_id"`
	Type        string  `json:"type"` // TOLL, PARKING, etc.
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Date        string  `json:"date"`
	Notes       *string `json:"notes"`
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

	exp := &models.DriveExpense{
		VehicleID:   vehicleID,
		TripGroupID: req.TripGroupID,
		DriveID:     req.DriveID,
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
