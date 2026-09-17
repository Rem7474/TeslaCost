package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/middleware"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// SaveOwnershipRequest is the payload of a vehicle ownership contract.
type SaveOwnershipRequest struct {
	AcquisitionType string   `json:"acquisition_type"`
	StartDate       string   `json:"start_date"`
	StartOdometer   *float64 `json:"start_odometer"`

	PurchasePrice         *money.Cents `json:"purchase_price"`
	PurchaseFees          *money.Cents `json:"purchase_fees"`
	Incentives            *money.Cents `json:"incentives"`
	ExpectedResaleValue   *money.Cents `json:"expected_resale_value"`
	ExpectedHoldingMonths *int         `json:"expected_holding_months"`

	LoanAmount           *money.Cents `json:"loan_amount"`
	LoanRatePct          *float64     `json:"loan_rate_pct"`
	LoanDurationMonths   *int         `json:"loan_duration_months"`
	LoanFees             *money.Cents `json:"loan_fees"`
	LoanInsuranceMonthly *money.Cents `json:"loan_insurance_monthly"`

	LeaseDownPayment         *money.Cents `json:"lease_down_payment"`
	LeaseMonthlyRent         *money.Cents `json:"lease_monthly_rent"`
	LeaseDurationMonths      *int         `json:"lease_duration_months"`
	LeaseFees                *money.Cents `json:"lease_fees"`
	LeaseDeposit             *money.Cents `json:"lease_deposit"`
	LeaseKmAllowancePerYear  *float64     `json:"lease_km_allowance_per_year"`
	LeaseExcessKmPrice       *float64     `json:"lease_excess_km_price"`
	LeaseEndFeesEstimate     *money.Cents `json:"lease_end_fees_estimate"`
	LeasePurchaseOptionPrice *money.Cents `json:"lease_purchase_option_price"`
	LeaseIncludesMaintenance bool         `json:"lease_includes_maintenance"`
	LeaseIncludesInsurance   bool         `json:"lease_includes_insurance"`
	LeaseIncludesTires       bool         `json:"lease_includes_tires"`
	OptionExercisedDate      *string      `json:"option_exercised_date"`

	EndDate   *string      `json:"end_date"`
	SalePrice *money.Cents `json:"sale_price"`
}

func validateOptionalAmounts(amounts ...*money.Cents) error {
	for _, a := range amounts {
		if a != nil {
			if err := validateAmount(*a, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateMonths(v *int, label string, required bool) error {
	if v == nil {
		if required {
			return errors.New(label + " est requise")
		}
		return nil
	}
	if *v <= 0 || *v > 360 {
		return errors.New(label + " doit être comprise entre 1 et 360 mois")
	}
	return nil
}

// buildOwnership validates a contract; fields that do not apply to the acquisition type are ignored.
func buildOwnership(vehicleID string, req *SaveOwnershipRequest) (*models.VehicleOwnership, error) {
	o := &models.VehicleOwnership{VehicleID: vehicleID, AcquisitionType: strings.ToUpper(strings.TrimSpace(req.AcquisitionType))}
	switch o.AcquisitionType {
	case models.AcquisitionCash, models.AcquisitionLoan, models.AcquisitionLOA, models.AcquisitionLLD:
	default:
		return nil, errors.New("mode d'acquisition invalide (CASH, LOAN, LOA ou LLD)")
	}

	start, err := parseDate(req.StartDate)
	if err != nil {
		return nil, errors.New("la date de début (achat ou contrat) est requise")
	}
	o.StartDate = start
	if req.StartOdometer != nil {
		if err := validateQuantity(*req.StartOdometer, 2_000_000); err != nil {
			return nil, errors.New("odomètre de début invalide")
		}
		o.StartOdometer = req.StartOdometer
	}

	end, err := parseOptionalDate(req.EndDate)
	if err != nil {
		return nil, err
	}
	if end != nil && end.Before(start) {
		return nil, errors.New("la fin de détention précède le début du contrat")
	}
	o.EndDate = end
	if err := validateOptionalAmounts(req.SalePrice, req.ExpectedResaleValue); err != nil {
		return nil, err
	}

	owned := func() error {
		if err := validateMonths(req.ExpectedHoldingMonths, "La durée de détention", false); err != nil {
			return err
		}
		o.ExpectedResaleValue, o.ExpectedHoldingMonths = req.ExpectedResaleValue, req.ExpectedHoldingMonths
		if req.SalePrice != nil {
			if end == nil {
				return errors.New("un prix de revente requiert la date de fin de détention")
			}
			o.SalePrice = req.SalePrice
		}
		return nil
	}

	switch o.AcquisitionType {
	case models.AcquisitionCash, models.AcquisitionLoan:
		if req.PurchasePrice == nil || *req.PurchasePrice <= 0 {
			return nil, errors.New("le prix d'achat est requis")
		}
		if err := validateOptionalAmounts(req.PurchasePrice, req.PurchaseFees, req.Incentives); err != nil {
			return nil, err
		}
		o.PurchasePrice, o.PurchaseFees, o.Incentives = req.PurchasePrice, req.PurchaseFees, req.Incentives
		net := *req.PurchasePrice + centsValue(req.PurchaseFees) - centsValue(req.Incentives)
		if req.ExpectedResaleValue != nil && *req.ExpectedResaleValue > net {
			return nil, errors.New("la valeur de revente estimée dépasse le coût d'achat net des aides")
		}
		if err := owned(); err != nil {
			return nil, err
		}

		if o.AcquisitionType == models.AcquisitionLoan {
			if req.LoanAmount == nil || *req.LoanAmount <= 0 {
				return nil, errors.New("le montant emprunté est requis")
			}
			if err := validateOptionalAmounts(req.LoanAmount, req.LoanFees, req.LoanInsuranceMonthly); err != nil {
				return nil, err
			}
			if *req.LoanAmount > *req.PurchasePrice+centsValue(req.PurchaseFees) {
				return nil, errors.New("le montant emprunté dépasse le coût d'achat")
			}
			if req.LoanRatePct == nil || *req.LoanRatePct < 0 || *req.LoanRatePct > 30 {
				return nil, errors.New("le taux du crédit doit être compris entre 0 et 30 %")
			}
			if err := validateMonths(req.LoanDurationMonths, "La durée du crédit", true); err != nil {
				return nil, err
			}
			o.LoanAmount, o.LoanRatePct, o.LoanDurationMonths = req.LoanAmount, req.LoanRatePct, req.LoanDurationMonths
			o.LoanFees, o.LoanInsuranceMonthly = req.LoanFees, req.LoanInsuranceMonthly
		}

	case models.AcquisitionLOA, models.AcquisitionLLD:
		if req.LeaseMonthlyRent == nil || *req.LeaseMonthlyRent <= 0 {
			return nil, errors.New("le loyer mensuel est requis")
		}
		if err := validateMonths(req.LeaseDurationMonths, "La durée du contrat", true); err != nil {
			return nil, err
		}
		if err := validateOptionalAmounts(req.LeaseMonthlyRent, req.LeaseDownPayment, req.LeaseFees, req.LeaseDeposit,
			req.LeaseEndFeesEstimate, req.LeasePurchaseOptionPrice); err != nil {
			return nil, err
		}
		if req.LeaseKmAllowancePerYear != nil {
			if err := validateQuantity(*req.LeaseKmAllowancePerYear, 200_000); err != nil {
				return nil, errors.New("forfait kilométrique invalide")
			}
		}
		if req.LeaseExcessKmPrice != nil {
			if err := validateQuantity(*req.LeaseExcessKmPrice, 5); err != nil {
				return nil, errors.New("prix du kilomètre supplémentaire invalide (0 à 5 €/km)")
			}
		}
		o.LeaseMonthlyRent, o.LeaseDurationMonths, o.LeaseDownPayment = req.LeaseMonthlyRent, req.LeaseDurationMonths, req.LeaseDownPayment
		o.LeaseFees, o.LeaseDeposit, o.LeaseEndFeesEstimate = req.LeaseFees, req.LeaseDeposit, req.LeaseEndFeesEstimate
		o.LeaseKmAllowancePerYear, o.LeaseExcessKmPrice = req.LeaseKmAllowancePerYear, req.LeaseExcessKmPrice
		o.LeaseIncludesMaintenance, o.LeaseIncludesInsurance, o.LeaseIncludesTires =
			req.LeaseIncludesMaintenance, req.LeaseIncludesInsurance, req.LeaseIncludesTires

		if o.AcquisitionType == models.AcquisitionLOA {
			o.LeasePurchaseOptionPrice = req.LeasePurchaseOptionPrice
			exercised, err := parseOptionalDate(req.OptionExercisedDate)
			if err != nil {
				return nil, err
			}
			if exercised != nil {
				if exercised.Before(start) || (end != nil && end.Before(*exercised)) {
					return nil, errors.New("la date de levée de l'option doit être comprise dans la période de détention")
				}
				if req.LeasePurchaseOptionPrice == nil {
					return nil, errors.New("la levée de l'option requiert le prix de l'option d'achat")
				}
				o.OptionExercisedDate = exercised
				if err := owned(); err != nil {
					return nil, err
				}
			}
		}
		if o.OptionExercisedDate == nil && req.SalePrice != nil {
			return nil, errors.New("un véhicule loué sans levée d'option ne peut pas avoir de prix de revente")
		}
	}
	return o, nil
}

func centsValue(c *money.Cents) money.Cents {
	if c == nil {
		return 0
	}
	return *c
}

// GetOwnership returns the ownership contract of a vehicle (404 when none is configured).
func (h *VehicleHandler) GetOwnership(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")
	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}
	if v.Role != models.RoleOwner {
		writeError(w, http.StatusForbidden, "Seul le propriétaire du véhicule peut consulter ou modifier le contrat d'acquisition")
		return
	}
	o, err := h.repo.GetVehicleOwnership(r.Context(), vehicleID)
	if err != nil {
		writeRepoError(w, err, "Failed to load ownership")
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// SaveOwnership creates or replaces the ownership contract of a vehicle.
func (h *VehicleHandler) SaveOwnership(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")
	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}
	if v.Role != models.RoleOwner {
		writeError(w, http.StatusForbidden, "Seul le propriétaire du véhicule peut consulter ou modifier le contrat d'acquisition")
		return
	}

	var req SaveOwnershipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	o, err := buildOwnership(vehicleID, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.repo.SaveVehicleOwnership(r.Context(), o); err != nil {
		writeRepoError(w, err, "Failed to save ownership")
		return
	}
	writeJSON(w, http.StatusOK, o)
}

// DeleteOwnership removes the ownership contract of a vehicle.
func (h *VehicleHandler) DeleteOwnership(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	vehicleID := chi.URLParam(r, "id")
	v, err := h.repo.GetVehicleByID(r.Context(), vehicleID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Vehicle not found")
		return
	}
	if v.Role != models.RoleOwner {
		writeError(w, http.StatusForbidden, "Seul le propriétaire du véhicule peut consulter ou modifier le contrat d'acquisition")
		return
	}
	if err := h.repo.DeleteVehicleOwnership(r.Context(), vehicleID); err != nil && !errors.Is(err, database.ErrNotFound) {
		writeRepoError(w, err, "Failed to delete ownership")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
