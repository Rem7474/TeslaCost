package handlers

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/database"
)

// maxAmount is the largest value storable in NUMERIC(10, 2).
const maxAmount = 99_999_999.99

var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)

var maintenanceCategories = map[string]bool{
	"MAINTENANCE":  true,
	"INSURANCE":    true,
	"SUBSCRIPTION": true,
	"TAX":          true,
	"ACCESSORY":    true,
	"OTHER":        true,
}

var driveExpenseTypes = map[string]bool{
	"TOLL":    true,
	"PARKING": true,
	"FERRY":   true,
	"OTHER":   true,
}

// writeRepoError maps repository errors to HTTP responses without leaking internal details.
func writeRepoError(w http.ResponseWriter, err error, fallback string) {
	var validationErr *database.ValidationError
	switch {
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Message)
	case errors.Is(err, database.ErrNotFound):
		writeError(w, http.StatusNotFound, "Élément introuvable")
	case errors.Is(err, database.ErrForeignReference):
		writeError(w, http.StatusBadRequest, "Référence invalide : trajet, groupe ou pneu inexistant pour ce véhicule")
	default:
		log.Printf("[api] %s: %v", fallback, err)
		writeError(w, http.StatusInternalServerError, fallback)
	}
}

// parseDate accepts RFC3339 timestamps and YYYY-MM-DD dates.
func parseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("date invalide : %q", value)
}

// parseOptionalDate parses an optional date; empty values yield nil.
func parseOptionalDate(value *string) (*time.Time, error) {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil, nil
	}
	t, err := parseDate(*value)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func validateAmount(amount float64, allowZero bool) error {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount > maxAmount {
		return errors.New("montant invalide")
	}
	if amount < 0 || (!allowZero && amount == 0) {
		return errors.New("le montant doit être positif")
	}
	return nil
}

// normalizeCurrency defaults to EUR and requires a conversion rate to EUR for other currencies.
func normalizeCurrency(currency string, fxRate *float64) (string, *float64, error) {
	cur := strings.ToUpper(strings.TrimSpace(currency))
	if cur == "" {
		cur = "EUR"
	}
	if !currencyPattern.MatchString(cur) {
		return "", nil, fmt.Errorf("devise invalide : %q", currency)
	}
	if cur == "EUR" {
		return cur, nil, nil
	}
	if fxRate == nil || math.IsNaN(*fxRate) || math.IsInf(*fxRate, 0) || *fxRate <= 0 {
		return "", nil, fmt.Errorf("un taux de conversion vers l'euro est requis pour une dépense en %s", cur)
	}
	return cur, fxRate, nil
}
