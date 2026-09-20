package services

import (
	"fmt"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// computeMonthlyFinancingAmortization spreads one-off lease down payment and application fees
// (or loan fees) over the contract duration, so the first month does not spike cost/km.
func (s *TCOService) computeMonthlyFinancingAmortization(ownership *models.VehicleOwnership, monthlyMap map[string]*MonthlyCost, now time.Time) {
	for _, mc := range monthlyMap {
		mc.FinancingAmortized = mc.Financing
	}

	if ownership == nil {
		return
	}

	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		loc = time.UTC
	}

	if ownership.IsLease() && ownership.LeaseDurationMonths != nil && *ownership.LeaseDurationMonths > 0 {
		prepaid := centsOrZero(ownership.LeaseDownPayment) + centsOrZero(ownership.LeaseFees)
		duration := *ownership.LeaseDurationMonths
		if prepaid > 0 && duration > 0 {
			monthlyPrepaid := money.FromFloat(prepaid.Float() / float64(duration))

			startInTz := ownership.StartDate.In(loc)
			startYear := startInTz.Year()
			startMonth := int(startInTz.Month())

			var endYearMonth string
			if ownership.EndDate != nil {
				endYearMonth = ownership.EndDate.In(loc).Format("2006-01")
			}
			var optionYearMonth string
			if ownership.OptionExercisedDate != nil {
				optionYearMonth = ownership.OptionExercisedDate.In(loc).Format("2006-01")
			}

			var cumPrepaid money.Cents
			for k := 0; k < duration; k++ {
				totalM := (startMonth - 1) + k
				y := startYear + totalM/12
				m := (totalM % 12) + 1
				monthStr := fmt.Sprintf("%04d-%02d", y, m)

				if endYearMonth != "" && monthStr > endYearMonth {
					break
				}
				if optionYearMonth != "" && monthStr >= optionYearMonth {
					break
				}

				mc, exists := monthlyMap[monthStr]
				if !exists {
					continue
				}

				var share money.Cents
				if k == duration-1 {
					share = prepaid - cumPrepaid
				} else {
					share = monthlyPrepaid
				}
				cumPrepaid += share

				if k == 0 {
					if mc.Financing >= prepaid {
						mc.FinancingAmortized = mc.Financing - prepaid + share
					} else {
						mc.FinancingAmortized = share
					}
				} else {
					mc.FinancingAmortized = mc.Financing + share
				}
			}
		}
	} else if ownership.AcquisitionType == models.AcquisitionLoan && ownership.LoanDurationMonths != nil && *ownership.LoanDurationMonths > 0 {
		loanFees := centsOrZero(ownership.LoanFees)
		duration := *ownership.LoanDurationMonths
		if loanFees > 0 && duration > 0 {
			monthlyFee := money.FromFloat(loanFees.Float() / float64(duration))

			startInTz := ownership.StartDate.In(loc)
			startYear := startInTz.Year()
			startMonth := int(startInTz.Month())

			var endYearMonth string
			if ownership.EndDate != nil {
				endYearMonth = ownership.EndDate.In(loc).Format("2006-01")
			}

			var cumFees money.Cents
			for k := 0; k < duration; k++ {
				totalM := (startMonth - 1) + k
				y := startYear + totalM/12
				m := (totalM % 12) + 1
				monthStr := fmt.Sprintf("%04d-%02d", y, m)

				if endYearMonth != "" && monthStr > endYearMonth {
					break
				}

				mc, exists := monthlyMap[monthStr]
				if !exists {
					continue
				}

				var share money.Cents
				if k == duration-1 {
					share = loanFees - cumFees
				} else {
					share = monthlyFee
				}
				cumFees += share

				if k == 0 {
					if mc.Financing >= loanFees {
						mc.FinancingAmortized = mc.Financing - loanFees + share
					} else {
						mc.FinancingAmortized = share
					}
				} else {
					mc.FinancingAmortized = mc.Financing + share
				}
			}
		}
	}
}
