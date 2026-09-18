package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/money"
)

// MaxBulkTollDrives bounds a bulk application: each drive triggers a TeslaMateAPI call.
const MaxBulkTollDrives = 100

// ApplyStatus is the outcome of applying the estimated toll to one drive.
type ApplyStatus string

const (
	ApplyCreated        ApplyStatus = "created"
	ApplyUpdated        ApplyStatus = "updated"
	ApplySkippedManual  ApplyStatus = "skipped_manual"
	ApplySkippedGroup   ApplyStatus = "skipped_trip_group"
	ApplySkippedNoPrice ApplyStatus = "skipped_no_price"
	ApplySkippedNoGPS   ApplyStatus = "skipped_no_gps"
	ApplyFailed         ApplyStatus = "failed"
)

// ApplyResult reports what happened for one drive.
type ApplyResult struct {
	DriveID string       `json:"drive_id"`
	Status  ApplyStatus  `json:"status"`
	Amount  *money.Cents `json:"amount,omitempty"`
	Error   string       `json:"error,omitempty"`
}

// BulkApplyResult aggregates per-drive outcomes.
type BulkApplyResult struct {
	Created          int           `json:"created"`
	Updated          int           `json:"updated"`
	SkippedManual    int           `json:"skipped_manual"`
	SkippedTripGroup int           `json:"skipped_trip_group"`
	SkippedNoPrice   int           `json:"skipped_no_price"`
	SkippedNoGPS     int           `json:"skipped_no_gps"`
	Failed           int           `json:"failed"`
	Results          []ApplyResult `json:"results"`
}

type tollAction int

const (
	tollActionCreate tollAction = iota
	tollActionUpdate
	tollActionSkipManual
	tollActionSkipGroup
)

// decideTollAction inspects the TOLL expenses already covering a drive. A manual toll is never
// overwritten, trip-group tolls are never touched, and an earlier auto toll is updated in place.
func decideTollAction(existing []models.DriveExpense) (tollAction, string) {
	var autoID string
	for _, e := range existing {
		if e.Type != "TOLL" {
			continue
		}
		if e.TripGroupID != nil {
			return tollActionSkipGroup, ""
		}
		if e.Source != models.ExpenseSourceAutoToll {
			return tollActionSkipManual, ""
		}
		autoID = e.ID
	}
	if autoID != "" {
		return tollActionUpdate, autoID
	}
	return tollActionCreate, ""
}

// sumEstimatedPrice totals the priced segments; ok is false when none has a price.
func sumEstimatedPrice(segments []models.TollSegment) (money.Cents, bool) {
	var total money.Cents
	priced := false
	for _, s := range segments {
		if s.EstimatedPrice != nil {
			total += *s.EstimatedPrice
			priced = true
		}
	}
	return total, priced && total > 0
}

// autoTollNotes describes the detected crossings, e.g. "Péage auto : A → B, Barrière : C".
func autoTollNotes(segments []models.TollSegment) string {
	parts := make([]string, 0, len(segments))
	for _, s := range segments {
		switch {
		case s.Type == "close" && s.Exit != nil:
			parts = append(parts, s.Entry+" → "+*s.Exit)
		case s.Type == "close":
			parts = append(parts, s.Entry+" (sortie non identifiée)")
		default:
			parts = append(parts, "Barrière "+s.Entry)
		}
	}
	return "Péage auto : " + strings.Join(parts, ", ")
}

// ApplyTollEstimate re-detects the drive's tolls and records the estimated total as an
// AUTO_TOLL expense, without ever overwriting a manual or trip-group toll.
func (s *TollDetectionService) ApplyTollEstimate(ctx context.Context, vehicle *models.Vehicle, driveID string) (ApplyResult, error) {
	res := ApplyResult{DriveID: driveID}

	drive, err := s.repo.GetDriveByID(ctx, driveID, vehicle.ID)
	if err != nil {
		return res, err
	}

	existing, err := s.repo.GetDriveExpensesByDriveID(ctx, vehicle.ID, driveID)
	if err != nil {
		return res, err
	}
	action, expenseID := decideTollAction(existing)
	switch action {
	case tollActionSkipManual:
		res.Status = ApplySkippedManual
		return res, nil
	case tollActionSkipGroup:
		res.Status = ApplySkippedGroup
		return res, nil
	}

	detection, err := s.DetectTolls(ctx, vehicle, driveID)
	if err != nil {
		if errors.Is(err, ErrNoGPSTrace) {
			res.Status = ApplySkippedNoGPS
			return res, nil
		}
		return res, err
	}

	total, ok := sumEstimatedPrice(detection.Segments)
	if !ok {
		res.Status = ApplySkippedNoPrice
		return res, nil
	}

	notes := autoTollNotes(detection.Segments)
	exp := &models.DriveExpense{
		ID:        expenseID,
		VehicleID: vehicle.ID,
		DriveID:   &drive.ID,
		Type:      "TOLL",
		Amount:    total,
		Currency:  "EUR",
		Date:      drive.StartTime,
		Notes:     &notes,
		Source:    models.ExpenseSourceAutoToll,
	}
	if err := s.repo.SaveDriveExpense(ctx, exp, nil, ""); err != nil {
		return res, fmt.Errorf("failed to save auto toll expense: %w", err)
	}

	res.Amount = &total
	res.Status = ApplyCreated
	if action == tollActionUpdate {
		res.Status = ApplyUpdated
	}
	return res, nil
}

// ApplyTollEstimatesBulk applies the estimate to each drive in turn; a failure on one drive
// is recorded and does not stop the others.
func (s *TollDetectionService) ApplyTollEstimatesBulk(ctx context.Context, vehicle *models.Vehicle, driveIDs []string) BulkApplyResult {
	out := BulkApplyResult{Results: make([]ApplyResult, 0, len(driveIDs))}
	for _, id := range driveIDs {
		res, err := s.ApplyTollEstimate(ctx, vehicle, id)
		if err != nil {
			res.Status = ApplyFailed
			if errors.Is(err, database.ErrNotFound) {
				res.Error = "Trajet introuvable"
			} else {
				res.Error = "Échec de la détection ou de l'enregistrement"
			}
		}
		switch res.Status {
		case ApplyCreated:
			out.Created++
		case ApplyUpdated:
			out.Updated++
		case ApplySkippedManual:
			out.SkippedManual++
		case ApplySkippedGroup:
			out.SkippedTripGroup++
		case ApplySkippedNoPrice:
			out.SkippedNoPrice++
		case ApplySkippedNoGPS:
			out.SkippedNoGPS++
		default:
			out.Failed++
		}
		out.Results = append(out.Results, res)
	}
	return out
}
