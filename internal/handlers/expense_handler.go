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
	"github.com/teslacost/teslacost/internal/storage"
)

type ExpenseHandler struct {
	repo           *database.Repository
	storageService *storage.FileStorageService
}

func NewExpenseHandler(repo *database.Repository, storageService *storage.FileStorageService) *ExpenseHandler {
	return &ExpenseHandler{repo: repo, storageService: storageService}
}

type CreateDriveExpenseRequest struct {
	TripGroupID *string     `json:"trip_group_id"`
	DriveID     *string     `json:"drive_id"`
	DriveIDs    []string    `json:"drive_ids"`
	Type        string      `json:"type"` // TOLL, PARKING, etc.
	Amount      money.Cents `json:"amount"`
	Currency    string      `json:"currency"`
	FxRate      *float64    `json:"fx_rate"`
	Date        string      `json:"date"`
	Notes       *string     `json:"notes"`
	DocumentID  *string     `json:"document_id"`
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
		DocumentID:  req.DocumentID,
	}, nil
}

func expenseGroupName(notes *string) string {
	if notes != nil && strings.TrimSpace(*notes) != "" {
		return *notes
	}
	return "Trajet multi-étapes"
}

func (h *ExpenseHandler) CreateDriveExpense(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
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
		writeRepoError(w, r, err, "Failed to record expense")
		return
	}

	writeJSON(w, http.StatusCreated, exp)
}

func (h *ExpenseHandler) ListDriveExpenses(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleViewer); v == nil {
		return
	}

	list, err := h.repo.ListDriveExpenses(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, r, err, "Failed to list expenses")
		return
	}
	if list == nil {
		list = []models.DriveExpense{}
	}

	writeJSON(w, http.StatusOK, list)
}

func (h *ExpenseHandler) UpdateDriveExpense(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	expenseID := chi.URLParam(r, "expenseId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
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
		writeRepoError(w, r, err, "Failed to update expense")
		return
	}

	writeJSON(w, http.StatusOK, exp)
}

func (h *ExpenseHandler) DeleteDriveExpense(w http.ResponseWriter, r *http.Request) {
	vehicleID := chi.URLParam(r, "vehicleId")
	expenseID := chi.URLParam(r, "expenseId")
	if v := requireVehicleAccess(w, r, h.repo, vehicleID, models.RoleEditor); v == nil {
		return
	}

	if err := h.repo.DeleteDriveExpense(r.Context(), vehicleID, expenseID); err != nil {
		writeRepoError(w, r, err, "Failed to delete expense")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Expense deleted successfully"})
}
