package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

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
