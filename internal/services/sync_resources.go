package services

import (
	"context"
	"fmt"
	"time"
)

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
