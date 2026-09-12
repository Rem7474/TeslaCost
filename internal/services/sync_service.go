package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
	"github.com/teslacost/teslacost/internal/teslamate"
)

// SyncResult returns metrics about what was synchronized.
type SyncResult struct {
	CurrentOdometer float64 `json:"current_odometer"`
	DrivesSynced    int     `json:"drives_synced"`
	ChargesSynced   int     `json:"charges_synced"`
	SyncedAt        string  `json:"synced_at"`
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
		return nil, fmt.Errorf("failed to fetch status from teslamate: %w", err)
	}

	return status, nil
}

// SyncVehicle runs a full sync cycle for a specific vehicle.
func (s *SyncService) SyncVehicle(ctx context.Context, v *models.Vehicle) (*SyncResult, error) {
	if v.TeslaMateAPIURL == nil || *v.TeslaMateAPIURL == "" {
		return nil, fmt.Errorf("vehicle does not have a TeslaMate API URL configured")
	}

	client, err := s.buildClient(v)
	if err != nil {
		return nil, err
	}

	carID := 1
	if v.TeslaMateCarID != nil && *v.TeslaMateCarID > 0 {
		carID = *v.TeslaMateCarID
	}

	// 1. Sync live Status & Odometer
	status, units, err := client.GetCarStatus(ctx, carID)
	if err != nil {
		log.Printf("[sync] Warning: Could not fetch car status: %v", err)
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

	// 2. Sync Drives (fetch latest 200 drives)
	driveList, driveUnits, err := client.GetDrives(ctx, carID, teslamate.DriveFilterOptions{
		Page: 1,
		Show: 200,
	})
	drivesCount := 0
	if err != nil {
		log.Printf("[sync] Warning: Could not fetch drives: %v", err)
	} else {
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
	}

	// 3. Sync Charges (fetch latest 200 charges)
	chargeList, chargeUnits, err := client.GetCharges(ctx, carID, teslamate.ChargeFilterOptions{
		Page: 1,
		Show: 200,
	})
	chargesCount := 0
	if err != nil {
		log.Printf("[sync] Warning: Could not fetch charges: %v", err)
	} else {
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
	}

	return &SyncResult{
		CurrentOdometer: v.CurrentOdometer,
		DrivesSynced:    drivesCount,
		ChargesSynced:   chargesCount,
		SyncedAt:        time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (s *SyncService) buildClient(v *models.Vehicle) (*teslamate.Client, error) {
	if v.TeslaMateAPIURL == nil || *v.TeslaMateAPIURL == "" {
		return nil, fmt.Errorf("no TeslaMate API URL provided")
	}

	cfg := teslamate.Config{
		BaseURL:  *v.TeslaMateAPIURL,
		AuthType: teslamate.AuthType(v.TeslaMateAuthType),
		Timeout:  20 * time.Second,
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
