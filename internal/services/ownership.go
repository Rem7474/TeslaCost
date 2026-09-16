package services

import (
	"fmt"
	"math"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// OwnershipCosts holds the economic (non-cash) adjustments derived from the ownership contract.
// Cash flows (purchase, rents, fees, interest) are entries of the cost ledger; these values turn them into
// a cost of ownership spread over time.
type OwnershipCosts struct {
	// Depreciation of an owned vehicle (purchase, or LOA after the purchase option is exercised).
	Depreciation money.Cents
	// LeasePrepaidAdjustment spreads the down payment and application fees over the contract duration:
	// negative while the contract runs (part of the cash paid upfront is not consumed yet).
	LeasePrepaidAdjustment money.Cents
	// LeaseEndFeesAdjustment accrues the estimated return fees over the contract duration.
	LeaseEndFeesAdjustment money.Cents
	// LeaseExcessKm is the excess-mileage penalty accrued so far against the pro-rata allowance.
	LeaseExcessKm money.Cents
	// LeaseExcessKmProjected is the penalty expected at the end of the contract at the current pace.
	LeaseExcessKmProjected money.Cents
	LeaseKmDriven          float64
	LeaseKmAllowanceToDate float64
	ContractEndDate        *time.Time

	IncludesMaintenance bool
	IncludesInsurance   bool
	IncludesTires       bool

	// Missing lists the settings needed for a complete cost of ownership.
	Missing []string
}

// monthsBetween returns the number of months (fractional) between two dates.
func monthsBetween(from, to time.Time) float64 {
	return to.Sub(from).Hours() / 24 / (365.25 / 12)
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func centsOrZero(c *money.Cents) money.Cents {
	if c == nil {
		return 0
	}
	return *c
}

// ComputeOwnershipCosts derives depreciation and lease adjustments at a given time.
// kmSinceStart is the distance driven since the start of the contract.
func ComputeOwnershipCosts(o *models.VehicleOwnership, now time.Time, kmSinceStart float64) OwnershipCosts {
	var c OwnershipCosts
	if o == nil {
		c.Missing = append(c.Missing, "Mode d'acquisition non renseigné (achat comptant, crédit, LOA ou LLD) : décote ou loyers absents du coût complet")
		return c
	}

	switch o.AcquisitionType {
	case models.AcquisitionCash, models.AcquisitionLoan:
		if o.PurchasePrice == nil || *o.PurchasePrice <= 0 {
			c.Missing = append(c.Missing, "Prix d'achat non renseigné")
		} else {
			base := *o.PurchasePrice + centsOrZero(o.PurchaseFees) - centsOrZero(o.Incentives)
			dep, ok := depreciation(base, o.StartDate, o, now)
			c.Depreciation = dep
			if !ok {
				c.Missing = append(c.Missing, "Valeur de revente estimée ou durée de détention non renseignée : décote exclue du coût complet")
			}
		}
		if o.AcquisitionType == models.AcquisitionLoan && (o.LoanAmount == nil || *o.LoanAmount <= 0 ||
			o.LoanDurationMonths == nil || *o.LoanDurationMonths <= 0 || o.LoanRatePct == nil) {
			c.Missing = append(c.Missing, "Crédit incomplet (montant emprunté, taux ou durée) : intérêts non calculés")
		}

	case models.AcquisitionLOA, models.AcquisitionLLD:
		computeLease(&c, o, now, kmSinceStart)
	}
	return c
}

// depreciation of an owned vehicle: realized at sale, otherwise spread linearly over the expected holding period.
func depreciation(base money.Cents, from time.Time, o *models.VehicleOwnership, now time.Time) (money.Cents, bool) {
	if o.EndDate != nil && !now.Before(*o.EndDate) && o.SalePrice != nil {
		return base - *o.SalePrice, true
	}
	if o.ExpectedResaleValue == nil || o.ExpectedHoldingMonths == nil || *o.ExpectedHoldingMonths <= 0 {
		return 0, false
	}
	until := now
	if o.EndDate != nil && o.EndDate.Before(until) {
		until = *o.EndDate
	}
	depreciable := base - *o.ExpectedResaleValue
	if depreciable <= 0 {
		return 0, true
	}
	ratio := clamp01(monthsBetween(from, until) / float64(*o.ExpectedHoldingMonths))
	return money.FromFloat(depreciable.Float() * ratio), true
}

func computeLease(c *OwnershipCosts, o *models.VehicleOwnership, now time.Time, kmSinceStart float64) {
	if o.LeaseDurationMonths != nil && *o.LeaseDurationMonths > 0 {
		contractEnd := o.StartDate.AddDate(0, *o.LeaseDurationMonths, 0)
		c.ContractEndDate = &contractEnd
	}
	if o.LeaseMonthlyRent == nil || *o.LeaseMonthlyRent <= 0 || o.LeaseDurationMonths == nil || *o.LeaseDurationMonths <= 0 {
		c.Missing = append(c.Missing, "Contrat de location incomplet (loyer mensuel ou durée) : loyers non comptés")
		return
	}
	duration := *o.LeaseDurationMonths
	contractEnd := *c.ContractEndDate

	// The lease phase ends at the contract term, at an early return, or when the purchase option is exercised.
	phaseEnd := contractEnd
	if o.EndDate != nil && o.EndDate.Before(phaseEnd) {
		phaseEnd = *o.EndDate
	}
	optionExercised := o.AcquisitionType == models.AcquisitionLOA && o.OptionExercisedDate != nil && !now.Before(*o.OptionExercisedDate)
	if o.OptionExercisedDate != nil && o.OptionExercisedDate.Before(phaseEnd) {
		phaseEnd = *o.OptionExercisedDate
	}
	started := !now.Before(o.StartDate)
	ended := !now.Before(phaseEnd)

	ratio := 0.0
	switch {
	case ended:
		ratio = 1
	case started:
		ratio = clamp01(monthsBetween(o.StartDate, now) / float64(duration))
	}

	inLease := started && !ended
	c.IncludesMaintenance = inLease && o.LeaseIncludesMaintenance
	c.IncludesInsurance = inLease && o.LeaseIncludesInsurance
	c.IncludesTires = inLease && o.LeaseIncludesTires

	if started {
		prepaid := centsOrZero(o.LeaseDownPayment) + centsOrZero(o.LeaseFees)
		c.LeasePrepaidAdjustment = money.FromFloat(prepaid.Float()*ratio) - prepaid
	}

	if o.LeaseEndFeesEstimate != nil && !optionExercised {
		accrued := money.FromFloat(o.LeaseEndFeesEstimate.Float() * ratio)
		var recognized money.Cents
		if ended {
			recognized = *o.LeaseEndFeesEstimate
		}
		c.LeaseEndFeesAdjustment = accrued - recognized
	}

	if o.LeaseKmAllowancePerYear == nil || o.LeaseExcessKmPrice == nil {
		c.Missing = append(c.Missing, "Forfait kilométrique ou prix du kilomètre supplémentaire non renseigné : pénalité de dépassement non estimée")
	} else if started && !optionExercised {
		elapsed := math.Min(monthsBetween(o.StartDate, now), monthsBetween(o.StartDate, phaseEnd))
		c.LeaseKmDriven = kmSinceStart
		c.LeaseKmAllowanceToDate = *o.LeaseKmAllowancePerYear * elapsed / 12
		c.LeaseExcessKm = money.FromFloat(math.Max(0, kmSinceStart-c.LeaseKmAllowanceToDate) * *o.LeaseExcessKmPrice)
		if elapsed >= 1 {
			projectedKm := kmSinceStart / elapsed * float64(duration)
			totalAllowance := *o.LeaseKmAllowancePerYear * float64(duration) / 12
			c.LeaseExcessKmProjected = money.FromFloat(math.Max(0, projectedKm-totalAllowance) * *o.LeaseExcessKmPrice)
		}
	}

	if o.AcquisitionType == models.AcquisitionLOA {
		if o.LeasePurchaseOptionPrice == nil {
			c.Missing = append(c.Missing, "Prix de l'option d'achat de la LOA non renseigné")
		}
		if optionExercised && o.LeasePurchaseOptionPrice != nil {
			dep, ok := depreciation(*o.LeasePurchaseOptionPrice, *o.OptionExercisedDate, o, now)
			c.Depreciation = dep
			if !ok {
				c.Missing = append(c.Missing, "Option d'achat levée : valeur de revente estimée ou durée de détention non renseignée, décote exclue")
			}
		}
	}
}

// LeaseWarnings returns informational warnings about the lease contract (mileage pace, contract end).
func (c OwnershipCosts) LeaseWarnings(now time.Time) []string {
	var warnings []string
	if c.LeaseExcessKmProjected > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"Au rythme actuel, le forfait kilométrique sera dépassé : pénalité estimée de %s € en fin de contrat", c.LeaseExcessKmProjected))
	}
	if c.ContractEndDate != nil && now.Before(*c.ContractEndDate) && monthsBetween(now, *c.ContractEndDate) <= 3 {
		warnings = append(warnings, fmt.Sprintf(
			"Fin du contrat de location le %s : pensez à l'option d'achat ou à la restitution", c.ContractEndDate.Format("02/01/2006")))
	}
	return warnings
}
