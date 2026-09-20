package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/teslamate"
)

// SyncResult returns metrics about what was synchronized.
type SyncResult struct {
	CurrentOdometer float64  `json:"current_odometer"`
	DrivesSynced    int      `json:"drives_synced"`
	DrivesAdded     int      `json:"drives_added"`
	DrivesUpdated   int      `json:"drives_updated"`
	ChargesSynced   int      `json:"charges_synced"`
	ChargesAdded    int      `json:"charges_added"`
	ChargesUpdated  int      `json:"charges_updated"`
	DrivesDeleted   int      `json:"drives_deleted_upstream"`
	ChargesDeleted  int      `json:"charges_deleted_upstream"`
	SyncedAt        string   `json:"synced_at"`
	Warnings        []string `json:"warnings,omitempty"`
}

const (
	syncPageSize = 50
	syncMaxPages = 500
	// Already imported records younger than this window are re-read on every incremental sync,
	// so that costs completed later in TeslaMate are picked up.
	resyncOverlap = 30 * 24 * time.Hour
	// Upsert failures reported individually before being summarized.
	maxDetailedSyncWarnings = 5
)

// syncStore is the persistence used by the synchronization.
type syncStore interface {
	UpdateVehicleOdometer(ctx context.Context, vehicleID string, odometer float64) error
	GetLatestTeslaMateDriveStartTime(ctx context.Context, vehicleID string) (*time.Time, error)
	GetLatestTeslaMateChargeDate(ctx context.Context, vehicleID string) (*time.Time, error)
	UpsertTeslaMateDrive(ctx context.Context, d *models.Drive) (bool, error)
	UpsertTeslaMateCharge(ctx context.Context, c *models.ChargeLog) (bool, error)
	IsFullImportCompleted(ctx context.Context, vehicleID, resource string) (bool, error)
	MarkSyncSuccess(ctx context.Context, vehicleID, resource string, fullImport bool) error
	ReconcileTeslaMateRecords(ctx context.Context, resource, vehicleID string, coveredAfter *time.Time, seenIDs []int) (*database.ReconcileResult, error)
	ListAllVehiclesWithTeslaMate(ctx context.Context) ([]models.Vehicle, error)
	UpsertBatterySnapshot(ctx context.Context, vehicleID string, day time.Time, snap models.BatterySnapshot) error
}

// SyncService orchestrates synchronization from TeslaMate to TeslaCost.
type SyncService struct {
	repo          syncStore
	encryptor     *crypto.Encryptor
	jobs          syncJobs
	notifications *NotificationService
	cbMu          sync.RWMutex
	breakers      map[string]*CircuitBreaker
}

// NewSyncService creates a new SyncService.
func NewSyncService(repo *database.Repository, encryptor *crypto.Encryptor) *SyncService {
	s := &SyncService{
		encryptor: encryptor,
		breakers:  make(map[string]*CircuitBreaker),
	}
	if repo != nil {
		s.repo = repo
	}
	return s
}

// SetNotificationService attaches a NotificationService to dispatch alerts on odometer updates.
func (s *SyncService) SetNotificationService(notifications *NotificationService) {
	s.notifications = notifications
}

func (s *SyncService) getCircuitBreaker(vehicleID string) *CircuitBreaker {
	s.cbMu.Lock()
	defer s.cbMu.Unlock()
	if s.breakers == nil {
		s.breakers = make(map[string]*CircuitBreaker)
	}
	cb, ok := s.breakers[vehicleID]
	if !ok {
		cb = NewCircuitBreaker()
		s.breakers[vehicleID] = cb
	}
	return cb
}

// GetCircuitBreaker returns the circuit breaker for a given vehicle.
func (s *SyncService) GetCircuitBreaker(vehicleID string) *CircuitBreaker {
	return s.getCircuitBreaker(vehicleID)
}

// SetCircuitBreaker overrides or sets a custom circuit breaker for testing or configuration.
func (s *SyncService) SetCircuitBreaker(vehicleID string, cb *CircuitBreaker) {
	s.cbMu.Lock()
	defer s.cbMu.Unlock()
	if s.breakers == nil {
		s.breakers = make(map[string]*CircuitBreaker)
	}
	s.breakers[vehicleID] = cb
}

// SyncVehicle runs a full sync cycle for a specific vehicle.
func (s *SyncService) SyncVehicle(ctx context.Context, v *models.Vehicle) (*SyncResult, error) {
	if v.TeslaMateAPIURL == nil || *v.TeslaMateAPIURL == "" {
		return nil, fmt.Errorf("le véhicule n'a pas d'URL TeslaMate configurée")
	}

	client, err := s.buildClient(v)
	if err != nil {
		return nil, err
	}

	carID := 1
	if v.TeslaMateCarID != nil && *v.TeslaMateCarID > 0 {
		carID = *v.TeslaMateCarID
	}

	var syncWarnings []string

	// 1. Sync live Status & Odometer
	status, units, err := client.GetCarStatus(ctx, carID)
	if err != nil {
		formattedErr := formatTeslaMateError(err, *v.TeslaMateAPIURL)
		slog.Error("error fetching car status", "component", "sync", "vehicle_id", v.ID, "error", formattedErr)
		return nil, fmt.Errorf("impossible de joindre TeslaMate (%s) : %w", *v.TeslaMateAPIURL, formattedErr)
	} else if status != nil {
		odometer := status.Odometer
		if units != nil {
			odometer = teslamate.ConvertDistanceToKm(odometer, units.UnitOfLength)
		}
		if odometer > v.CurrentOdometer {
			if s.repo != nil {
				if err := s.repo.UpdateVehicleOdometer(ctx, v.ID, odometer); err != nil {
					syncWarnings = append(syncWarnings, fmt.Sprintf("Odomètre : %v", err))
				}
			}
			v.CurrentOdometer = odometer
			if s.notifications != nil {
				go func(veh models.Vehicle, odo float64) {
					defer recoverPanic("sync.notifications")
					notifyCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					if err := s.notifications.CheckAndNotify(notifyCtx, &veh, odo); err != nil {
						slog.Error("CheckAndNotify failed", "component", "notification", "vehicle_id", veh.ID, "error", err)
					}
				}(*v, odometer)
			}
		} else if odometer > 0 && odometer+1 < v.CurrentOdometer {
			syncWarnings = append(syncWarnings, fmt.Sprintf(
				"Odomètre TeslaMate (%.0f km) inférieur à l'odomètre enregistré (%.0f km) : vérifiez la saisie manuelle du véhicule",
				odometer, v.CurrentOdometer))
		}
	}

	drives := s.syncDrives(ctx, client, v, carID)
	charges := s.syncCharges(ctx, client, v, carID)
	s.syncBatteryHealth(ctx, client, v, carID)
	syncWarnings = append(syncWarnings, drives.warnings...)
	syncWarnings = append(syncWarnings, charges.warnings...)

	// If there were warnings and 0 items synced at all: report as error
	if len(syncWarnings) > 0 && drives.count == 0 && charges.count == 0 {
		return nil, fmt.Errorf("échec de la synchronisation : %s", strings.Join(syncWarnings, " ; "))
	}

	return &SyncResult{
		CurrentOdometer: v.CurrentOdometer,
		DrivesSynced:    drives.count,
		DrivesAdded:     drives.added,
		DrivesUpdated:   drives.updated,
		ChargesSynced:   charges.count,
		ChargesAdded:    charges.added,
		ChargesUpdated:  charges.updated,
		DrivesDeleted:   drives.deletedUpstream,
		ChargesDeleted:  charges.deletedUpstream,
		SyncedAt:        time.Now().UTC().Format(time.RFC3339),
		Warnings:        syncWarnings,
	}, nil
}

// syncBatteryHealth keeps today's battery health as computed by TeslaMate. It is best effort: TeslaMateApi
// versions without the endpoint, or a vehicle with too little charging history, simply leave no snapshot.
func (s *SyncService) syncBatteryHealth(ctx context.Context, client *teslamate.Client, v *models.Vehicle, carID int) {
	if s.repo == nil {
		return
	}
	health, err := client.GetBatteryHealth(ctx, carID)
	if err != nil {
		slog.Info("battery health not available", "component", "sync", "vehicle_id", v.ID, "reason", err)
		return
	}
	if health.CurrentCapacity <= 0 && health.MaxCapacity <= 0 {
		return
	}
	positive := func(v float64) *float64 {
		if v <= 0 {
			return nil
		}
		return &v
	}
	snap := models.BatterySnapshot{
		MaxCapacityKwh:     positive(health.MaxCapacity),
		CurrentCapacityKwh: positive(health.CurrentCapacity),
		HealthPercent:      positive(health.BatteryHealthPercentage),
	}
	if err := s.repo.UpsertBatterySnapshot(ctx, v.ID, time.Now().UTC(), snap); err != nil {
		slog.Warn("could not store battery health", "component", "sync", "vehicle_id", v.ID, "error", err)
	}
}
