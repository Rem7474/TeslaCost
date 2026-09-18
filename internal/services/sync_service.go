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
	"github.com/teslacost/teslacost/internal/money"
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

// formatTeslaMateError enriches error messages with actionable troubleshooting hints.
func formatTeslaMateError(err error, rawURL string) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	isDockerLocalhost := strings.Contains(rawURL, "localhost") || strings.Contains(rawURL, "127.0.0.1")
	if isDockerLocalhost && (strings.Contains(msg, "connection refused") || strings.Contains(msg, "dial tcp")) {
		return fmt.Errorf("%s (Remarque : dans Docker, 'localhost' désigne le conteneur TeslaCost lui-même. Utilisez 'http://host.docker.internal:PORT' ou l'IP locale de votre machine)", msg)
	}
	if strings.Contains(msg, "Client.Timeout exceeded") || strings.Contains(msg, "context deadline exceeded") {
		return fmt.Errorf("%s (Délai d'attente dépassé : vérifiez que l'adresse et le port sont joignables et que TeslaMate répond)", msg)
	}
	return err
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

// TestConnection verifies if connection to TeslaMate works for a given vehicle config.
func (s *SyncService) TestConnection(ctx context.Context, v *models.Vehicle) (*teslamate.StatusDetails, error) {
	client, err := s.buildClient(v)
	if err != nil {
		return nil, err
	}

	carID := 1
	if v.TeslaMateCarID != nil && *v.TeslaMateCarID > 0 {
		carID = *v.TeslaMateCarID
	}

	status, _, err := client.GetCarStatus(ctx, carID)
	if err != nil {
		rawURL := ""
		if v.TeslaMateAPIURL != nil {
			rawURL = *v.TeslaMateAPIURL
		}
		return nil, formatTeslaMateError(err, rawURL)
	}

	return status, nil
}

// TestConnectionRaw verifies if connection to TeslaMate works using raw credentials without existing vehicle.
func (s *SyncService) TestConnectionRaw(ctx context.Context, apiURL string, authType models.AuthMode, apiKey, basicUser, basicPass string, carID int) (*teslamate.StatusDetails, error) {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		return nil, fmt.Errorf("l'URL de l'API TeslaMate est requise")
	}

	cfg := teslamate.Config{
		BaseURL:  apiURL,
		AuthType: teslamate.AuthType(authType),
		Timeout:  15 * time.Second,
	}

	switch cfg.AuthType {
	case teslamate.AuthBearer:
		cfg.APIToken = apiKey
	case teslamate.AuthBasic:
		cfg.Username = basicUser
		cfg.Password = basicPass
	}

	client, err := teslamate.NewClient(cfg)
	if err != nil {
		return nil, formatTeslaMateError(err, apiURL)
	}

	if carID <= 0 {
		carID = 1
	}

	status, _, err := client.GetCarStatus(ctx, carID)
	if err != nil {
		return nil, formatTeslaMateError(err, apiURL)
	}

	return status, nil
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

type resourceSyncStats struct {
	count, added, updated, failed, deletedUpstream int
	warnings                                       []string
	seenIDs                                        []int
	oldestSeen                                     *time.Time
}

func (st *resourceSyncStats) see(id int, start time.Time) {
	st.seenIDs = append(st.seenIDs, id)
	if st.oldestSeen == nil || start.Before(*st.oldestSeen) {
		t := start
		st.oldestSeen = &t
	}
}

// reconcile flags records deleted in TeslaMate over the window covered by a completed pass:
// the whole history for a full import, otherwise everything strictly newer than the oldest record read.
func (s *SyncService) reconcile(ctx context.Context, st *resourceSyncStats, vehicleID, resource, label string, fullPass bool) {
	coveredAfter := st.oldestSeen
	if fullPass {
		coveredAfter = nil
	} else if coveredAfter == nil {
		return
	}
	res, err := s.repo.ReconcileTeslaMateRecords(ctx, resource, vehicleID, coveredAfter, st.seenIDs)
	switch {
	case err != nil:
		st.warnings = append(st.warnings, fmt.Sprintf("%s : rapprochement avec TeslaMate impossible (%v)", label, err))
	case res.Skipped:
		st.warnings = append(st.warnings, fmt.Sprintf(
			"%s : %d éléments absents de TeslaMate, suppression ignorée par sécurité (vérifiez l'identifiant du véhicule TeslaMate)", label, res.Missing))
	case res.Marked > 0:
		st.deletedUpstream = res.Marked
		st.warnings = append(st.warnings, fmt.Sprintf("%s : %d élément(s) supprimé(s) dans TeslaMate, exclu(s) des calculs", label, res.Marked))
	}
}

func (st *resourceSyncStats) recordUpsert(isInserted bool, err error, label string) {
	if err != nil {
		st.failed++
		if st.failed <= maxDetailedSyncWarnings {
			st.warnings = append(st.warnings, fmt.Sprintf("%s non importé : %v", label, err))
		}
		return
	}
	st.count++
	if isInserted {
		st.added++
	} else {
		st.updated++
	}
}

func (st *resourceSyncStats) finalize(resourceLabel string) {
	if st.failed > maxDetailedSyncWarnings {
		st.warnings = append(st.warnings, fmt.Sprintf("%s : %d éléments non importés au total", resourceLabel, st.failed))
	}
}

// incrementalStopBefore returns the date before which pagination can stop, or nil when the complete
// history has never been imported successfully (an interrupted import is resumed from scratch).
func (s *SyncService) incrementalStopBefore(ctx context.Context, vehicleID, resource string, latest func(context.Context, string) (*time.Time, error)) *time.Time {
	fullImportDone, err := s.repo.IsFullImportCompleted(ctx, vehicleID, resource)
	if err != nil || !fullImportDone {
		return nil
	}
	latestTime, err := latest(ctx, vehicleID)
	if err != nil || latestTime == nil {
		return nil
	}
	stop := latestTime.Add(-resyncOverlap)
	return &stop
}

func (s *SyncService) syncDrives(ctx context.Context, client *teslamate.Client, v *models.Vehicle, carID int) resourceSyncStats {
	var st resourceSyncStats
	if s.repo == nil {
		return st
	}
	stopBefore := s.incrementalStopBefore(ctx, v.ID, "drives", s.repo.GetLatestTeslaMateDriveStartTime)

	completed := false
	for page := 1; page <= syncMaxPages; page++ {
		driveList, driveUnits, err := client.GetDrives(ctx, carID, teslamate.DriveFilterOptions{
			Page: page,
			Show: syncPageSize,
		})
		if err != nil {
			formattedErr := formatTeslaMateError(err, *v.TeslaMateAPIURL)
			slog.Warn("could not fetch drives", "component", "sync", "vehicle_id", v.ID, "page", page, "error", formattedErr)
			st.warnings = append(st.warnings, fmt.Sprintf("Trajets (page %d) : %v — l'import reprendra à la prochaine synchronisation", page, formattedErr))
			break
		}
		if len(driveList) == 0 {
			completed = true
			break
		}

		reachedKnownHistory := false
		for _, td := range driveList {
			startTime, err := td.ParsedStartTime()
			if err != nil || startTime.IsZero() {
				st.recordUpsert(false, fmt.Errorf("date de début invalide (%q)", td.StartDate), fmt.Sprintf("Trajet TeslaMate #%d", td.DriveID))
				continue
			}
			endTime, _ := td.ParsedEndTime()
			if endTime.IsZero() {
				endTime = startTime.Add(time.Duration(td.DurationMin) * time.Minute)
			}

			if stopBefore != nil && startTime.Before(*stopBefore) {
				reachedKnownHistory = true
			}
			st.see(td.DriveID, startTime)

			isInserted, err := s.repo.UpsertTeslaMateDrive(ctx, buildDrive(v.ID, td, driveUnits, startTime, endTime))
			st.recordUpsert(isInserted, err, fmt.Sprintf("Trajet TeslaMate #%d", td.DriveID))
		}

		if len(driveList) < syncPageSize || reachedKnownHistory {
			completed = true
			break
		}
		if page == syncMaxPages {
			st.warnings = append(st.warnings, fmt.Sprintf("Trajets : limite de %d pages atteinte, historique partiellement importé", syncMaxPages))
		}
	}

	st.finalize("Trajets")
	if completed && st.failed == 0 {
		s.reconcile(ctx, &st, v.ID, "drives", "Trajets", stopBefore == nil)
		if err := s.repo.MarkSyncSuccess(ctx, v.ID, "drives", true); err != nil {
			st.warnings = append(st.warnings, fmt.Sprintf("Trajets : état de synchronisation non enregistré (%v)", err))
		}
	}
	return st
}

func buildDrive(vehicleID string, td teslamate.Drive, units *teslamate.Units, startTime, endTime time.Time) *models.Drive {
	distKm := td.OdometerDetails.OdometerDistance
	startOdo := td.OdometerDetails.OdometerStart
	endOdo := td.OdometerDetails.OdometerEnd
	if units != nil {
		distKm = teslamate.ConvertDistanceToKm(distKm, units.UnitOfLength)
		startOdo = teslamate.ConvertDistanceToKm(startOdo, units.UnitOfLength)
		endOdo = teslamate.ConvertDistanceToKm(endOdo, units.UnitOfLength)
	}

	var speedAvg *float64
	if td.SpeedAvg > 0 {
		speedAvg = &td.SpeedAvg
	}

	var startAddr, endAddr *string
	if td.StartAddress != "" {
		startAddr = &td.StartAddress
	}
	if td.EndAddress != "" {
		endAddr = &td.EndAddress
	}

	var speedMax, powerMax, powerMin *int
	if td.SpeedMax > 0 {
		speedMax = &td.SpeedMax
	}
	if td.PowerMax != 0 {
		powerMax = &td.PowerMax
	}
	if td.PowerMin != 0 {
		powerMin = &td.PowerMin
	}

	tmDriveID := td.DriveID
	return &models.Drive{
		VehicleID:           vehicleID,
		TeslaMateDriveID:    &tmDriveID,
		StartTime:           startTime,
		EndTime:             endTime,
		StartOdometer:       &startOdo,
		EndOdometer:         &endOdo,
		DistanceKm:          distKm,
		DurationMin:         td.DurationMin,
		SpeedAvg:            speedAvg,
		SpeedMax:            speedMax,
		PowerMax:            powerMax,
		PowerMin:            powerMin,
		StartAddress:        startAddr,
		EndAddress:          endAddr,
		EnergyConsumedKwh:   td.EnergyConsumedNet,
		ConsumptionKwh100km: td.ConsumptionNet,
		Tags:                []string{},
	}
}

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
	}
}

func (s *SyncService) buildClient(v *models.Vehicle) (*teslamate.Client, error) {
	return buildTeslaMateClient(v, s.encryptor)
}

// buildTeslaMateClient constructs an authenticated TeslaMateAPI client for a vehicle,
// decrypting whichever credential its configured auth type requires. Shared by SyncService
// and any other service that needs to call TeslaMateAPI on a vehicle's behalf.
func buildTeslaMateClient(v *models.Vehicle, encryptor *crypto.Encryptor) (*teslamate.Client, error) {
	if v.TeslaMateAPIURL == nil || *v.TeslaMateAPIURL == "" {
		return nil, fmt.Errorf("no TeslaMate API URL provided")
	}

	cfg := teslamate.Config{
		BaseURL:  *v.TeslaMateAPIURL,
		AuthType: teslamate.AuthType(v.TeslaMateAuthType),
		Timeout:  60 * time.Second,
	}

	switch cfg.AuthType {
	case teslamate.AuthBearer:
		if v.TeslaMateAPIKeyEncrypted != nil && *v.TeslaMateAPIKeyEncrypted != "" {
			token, err := encryptor.Decrypt(*v.TeslaMateAPIKeyEncrypted)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt API key: %w", err)
			}
			cfg.APIToken = token
		}
	case teslamate.AuthBasic:
		if v.TeslaMateBasicUser != nil {
			cfg.Username = *v.TeslaMateBasicUser
		}
		if v.TeslaMateBasicPassEnc != nil && *v.TeslaMateBasicPassEnc != "" {
			pass, err := encryptor.Decrypt(*v.TeslaMateBasicPassEnc)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt basic password: %w", err)
			}
			cfg.Password = pass
		}
	}

	return teslamate.NewClient(cfg)
}

// StartBackgroundWorker runs periodic incremental sync for all vehicles with TeslaMate configured.
func (s *SyncService) StartBackgroundWorker(ctx context.Context, intervalMinutes int) {
	if intervalMinutes <= 0 {
		slog.Info("background auto-sync worker disabled (interval <= 0)", "component", "auto-sync")
		return
	}

	interval := time.Duration(intervalMinutes) * time.Minute
	slog.Info("background auto-sync worker started", "component", "auto-sync", "interval", interval.String())

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("background auto-sync worker stopped", "component", "auto-sync")
			return
		case <-ticker.C:
			s.runBackgroundSyncCycle(ctx)
		}
	}
}

func (s *SyncService) runBackgroundSyncCycle(ctx context.Context) {
	if s.repo == nil {
		return
	}

	vehicles, err := s.repo.ListAllVehiclesWithTeslaMate(ctx)
	if err != nil {
		slog.Error("failed to list vehicles", "component", "auto-sync", "error", err)
		return
	}

	for _, v := range vehicles {
		s.runScheduledSyncSafe(ctx, v)
	}
}

// runScheduledSyncSafe runs a single vehicle's scheduled sync, recovering from any panic
// so that one vehicle failing unexpectedly does not take down the whole background worker
// (and, by extension, the server process) for every other vehicle.
func (s *SyncService) runScheduledSyncSafe(ctx context.Context, v models.Vehicle) {
	defer recoverPanic(fmt.Sprintf("sync.scheduled(%s)", v.ID))
	s.runScheduledSync(ctx, v)
}
