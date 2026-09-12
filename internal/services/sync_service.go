package services

import (
	"context"
	"fmt"
	"log"
	"strings"
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
	ChargesSynced   int      `json:"charges_synced"`
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

// SyncService orchestrates synchronization from TeslaMate to TeslaCost.
type SyncService struct {
	repo      *database.Repository
	encryptor *crypto.Encryptor
}

// NewSyncService creates a new SyncService.
func NewSyncService(repo *database.Repository, encryptor *crypto.Encryptor) *SyncService {
	return &SyncService{
		repo:      repo,
		encryptor: encryptor,
	}
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
		log.Printf("[sync] Error fetching car status: %v", formattedErr)
		return nil, fmt.Errorf("impossible de joindre TeslaMate (%s) : %w", *v.TeslaMateAPIURL, formattedErr)
	} else if status != nil {
		odometer := status.Odometer
		if units != nil {
			odometer = teslamate.ConvertDistanceToKm(odometer, units.UnitOfLength)
		}
		if odometer > v.CurrentOdometer {
			_ = s.repo.UpdateVehicleOdometer(ctx, v.ID, odometer)
			v.CurrentOdometer = odometer
		}
	}

	// 2. Sync Drives in smaller batches of 50 up to 4 pages (max 200) to prevent slow SQL joins from timing out
	drivesCount := 0
	for page := 1; page <= 4; page++ {
		driveList, driveUnits, err := client.GetDrives(ctx, carID, teslamate.DriveFilterOptions{
			Page: page,
			Show: 50,
		})
		if err != nil {
			formattedErr := formatTeslaMateError(err, *v.TeslaMateAPIURL)
			log.Printf("[sync] Warning: Could not fetch drives (page %d): %v", page, formattedErr)
			syncWarnings = append(syncWarnings, fmt.Sprintf("Trajets : %v", formattedErr))
			break
		}
		if len(driveList) == 0 {
			break
		}

		for _, td := range driveList {
			startTime, _ := td.ParsedStartTime()
			endTime, _ := td.ParsedEndTime()
			if startTime.IsZero() {
				startTime = time.Now()
			}
			if endTime.IsZero() {
				endTime = startTime.Add(time.Duration(td.DurationMin) * time.Minute)
			}

			distKm := td.OdometerDetails.OdometerDistance
			startOdo := td.OdometerDetails.OdometerStart
			endOdo := td.OdometerDetails.OdometerEnd
			if driveUnits != nil {
				distKm = teslamate.ConvertDistanceToKm(distKm, driveUnits.UnitOfLength)
				startOdo = teslamate.ConvertDistanceToKm(startOdo, driveUnits.UnitOfLength)
				endOdo = teslamate.ConvertDistanceToKm(endOdo, driveUnits.UnitOfLength)
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

			tmDriveID := td.DriveID
			d := &models.Drive{
				VehicleID:           v.ID,
				TeslaMateDriveID:    &tmDriveID,
				StartTime:           startTime,
				EndTime:             endTime,
				StartOdometer:       &startOdo,
				EndOdometer:         &endOdo,
				DistanceKm:          distKm,
				DurationMin:         td.DurationMin,
				SpeedAvg:            speedAvg,
				StartAddress:        startAddr,
				EndAddress:          endAddr,
				EnergyConsumedKwh:   td.EnergyConsumedNet,
				ConsumptionKwh100km: td.ConsumptionNet,
				Tags:                []string{},
			}

			if err := s.repo.UpsertTeslaMateDrive(ctx, d); err == nil {
				drivesCount++
			}
		}

		if len(driveList) < 50 {
			break
		}
	}

	// 3. Sync Charges in smaller batches of 50 up to 4 pages (max 200)
	chargesCount := 0
	for page := 1; page <= 4; page++ {
		chargeList, chargeUnits, err := client.GetCharges(ctx, carID, teslamate.ChargeFilterOptions{
			Page: page,
			Show: 50,
		})
		if err != nil {
			formattedErr := formatTeslaMateError(err, *v.TeslaMateAPIURL)
			log.Printf("[sync] Warning: Could not fetch charges (page %d): %v", page, formattedErr)
			syncWarnings = append(syncWarnings, fmt.Sprintf("Recharges : %v", formattedErr))
			break
		}
		if len(chargeList) == 0 {
			break
		}

		for _, tc := range chargeList {
			startDate, _ := tc.ParsedStartTime()
			endDate, _ := tc.ParsedEndTime()
			if startDate.IsZero() {
				startDate = time.Now()
			}

			odo := tc.Odometer
			if chargeUnits != nil && odo > 0 {
				odo = teslamate.ConvertDistanceToKm(odo, chargeUnits.UnitOfLength)
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
			c := &models.ChargeLog{
				VehicleID:         v.ID,
				TeslaMateChargeID: &tmChargeID,
				Date:              startDate,
				EndDate:           endPtr,
				Address:           addrPtr,
				KwhAdded:          tc.ChargeEnergyAdded,
				KwhUsed:           kwhUsedPtr,
				Cost:              tc.Cost,
				Currency:          "EUR",
				Odometer:          odoPtr,
			}

			if err := s.repo.UpsertTeslaMateCharge(ctx, c); err == nil {
				chargesCount++
			}
		}

		if len(chargeList) < 50 {
			break
		}
	}

	// If there were warnings and 0 items synced at all: report as error
	if len(syncWarnings) > 0 && drivesCount == 0 && chargesCount == 0 {
		return nil, fmt.Errorf("échec de la synchronisation : %s", strings.Join(syncWarnings, " ; "))
	}

	return &SyncResult{
		CurrentOdometer: v.CurrentOdometer,
		DrivesSynced:    drivesCount,
		ChargesSynced:   chargesCount,
		SyncedAt:        time.Now().UTC().Format(time.RFC3339),
		Warnings:        syncWarnings,
	}, nil
}

func (s *SyncService) buildClient(v *models.Vehicle) (*teslamate.Client, error) {
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
			token, err := s.encryptor.Decrypt(*v.TeslaMateAPIKeyEncrypted)
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
			pass, err := s.encryptor.Decrypt(*v.TeslaMateBasicPassEnc)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt basic password: %w", err)
			}
			cfg.Password = pass
		}
	}

	return teslamate.NewClient(cfg)
}
