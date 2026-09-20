package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
	"github.com/teslacost/teslacost/internal/teslamate"
)

func (s *SyncService) syncCharges(ctx context.Context, client *teslamate.Client, v *models.Vehicle, carID int) resourceSyncStats {
	var st resourceSyncStats
	if s.repo == nil {
		return st
	}
	stopBefore := s.incrementalStopBefore(ctx, v.ID, "charges", s.repo.GetLatestTeslaMateChargeDate)

	completed := false
	for page := 1; page <= syncMaxPages; page++ {
		chargeList, chargeUnits, err := client.GetCharges(ctx, carID, teslamate.ChargeFilterOptions{
			Page: page,
			Show: syncPageSize,
		})
		if err != nil {
			formattedErr := formatTeslaMateError(err, *v.TeslaMateAPIURL)
			slog.Warn("could not fetch charges", "component", "sync", "vehicle_id", v.ID, "page", page, "error", formattedErr)
			st.warnings = append(st.warnings, fmt.Sprintf("Recharges (page %d) : %v — l'import reprendra à la prochaine synchronisation", page, formattedErr))
			break
		}
		if len(chargeList) == 0 {
			completed = true
			break
		}

		reachedKnownHistory := false
		for _, tc := range chargeList {
			startDate, err := tc.ParsedStartTime()
			if err != nil || startDate.IsZero() {
				st.recordUpsert(false, fmt.Errorf("date de début invalide (%q)", tc.StartDate), fmt.Sprintf("Recharge TeslaMate #%d", tc.ChargeID))
				continue
			}
			if stopBefore != nil && startDate.Before(*stopBefore) {
				reachedKnownHistory = true
			}
			st.see(tc.ChargeID, startDate)

			isInserted, err := s.repo.UpsertTeslaMateCharge(ctx, buildCharge(v.ID, tc, chargeUnits, startDate))
			st.recordUpsert(isInserted, err, fmt.Sprintf("Recharge TeslaMate #%d", tc.ChargeID))
		}

		if len(chargeList) < syncPageSize || reachedKnownHistory {
			completed = true
			break
		}
		if page == syncMaxPages {
			st.warnings = append(st.warnings, fmt.Sprintf("Recharges : limite de %d pages atteinte, historique partiellement importé", syncMaxPages))
		}
	}

	st.finalize("Recharges")
	if completed && st.failed == 0 {
		s.reconcile(ctx, &st, v.ID, "charges", "Recharges", stopBefore == nil)
		if err := s.repo.MarkSyncSuccess(ctx, v.ID, "charges", true); err != nil {
			st.warnings = append(st.warnings, fmt.Sprintf("Recharges : état de synchronisation non enregistré (%v)", err))
		}
	}
	return st
}

// costCents converts a TeslaMate cost; nil (no tariff configured) stays nil.
func costCents(cost *float64) *money.Cents {
	if cost == nil {
		return nil
	}
	c := money.FromFloat(*cost)
	return &c
}

func buildCharge(vehicleID string, tc teslamate.Charge, units *teslamate.Units, startDate time.Time) *models.ChargeLog {
	endDate, _ := tc.ParsedEndTime()

	odo := tc.Odometer
	if units != nil && odo > 0 {
		odo = teslamate.ConvertDistanceToKm(odo, units.UnitOfLength)
	}
	var odoPtr *float64
	if odo > 0 {
		odoPtr = &odo
	}

	var endPtr *time.Time
	if !endDate.IsZero() {
		endPtr = &endDate
	}

	var addrPtr *string
	if tc.Address != "" {
		addrPtr = &tc.Address
	}

	var kwhUsedPtr *float64
	if tc.ChargeEnergyUsed > 0 {
		kwhUsedPtr = &tc.ChargeEnergyUsed
	}

	startLevel, endLevel := batteryLevels(tc.BatteryDetails)

	tmChargeID := tc.ChargeID
	return &models.ChargeLog{
		VehicleID:         vehicleID,
		TeslaMateChargeID: &tmChargeID,
		Date:              startDate,
		EndDate:           endPtr,
		Address:           addrPtr,
		KwhAdded:          tc.ChargeEnergyAdded,
		KwhUsed:           kwhUsedPtr,
		Cost:              costCents(tc.Cost),
		CostSource:        "TESLAMATE",
		Currency:          "EUR",
		Odometer:          odoPtr,
		StartBatteryLevel: startLevel,
		EndBatteryLevel:   endLevel,
		OutsideTempC:      tempCelsius(tc.OutsideTempAvg, units),
	}
}
