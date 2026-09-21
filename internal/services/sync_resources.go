package services

import (
	"context"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
)

type resourceSyncStats struct {
	count, added, updated, failed, deletedUpstream int
	warnings                                       []*apierror.Message
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
func (s *SyncService) reconcile(ctx context.Context, st *resourceSyncStats, vehicleID, resource string, fullPass bool) {
	coveredAfter := st.oldestSeen
	if fullPass {
		coveredAfter = nil
	} else if coveredAfter == nil {
		return
	}
	res, err := s.repo.ReconcileTeslaMateRecords(ctx, resource, vehicleID, coveredAfter, st.seenIDs)
	switch {
	case err != nil:
		st.warnings = append(st.warnings, apierror.NewMessagef("sync.reconcile_failed", "%s: reconciliation with TeslaMate failed (%v)", "kw:"+resource, err))
	case res.Skipped:
		st.warnings = append(st.warnings, apierror.NewMessagef("sync.deletion_skipped",
			"%s: %d items missing from TeslaMate, deletion skipped for safety (check the TeslaMate vehicle identifier)", "kw:"+resource, res.Missing))
	case res.Marked > 0:
		st.deletedUpstream = res.Marked
		st.warnings = append(st.warnings, apierror.NewMessagef("sync.deleted_upstream", "%s: %d item(s) deleted in TeslaMate, excluded from the calculations", "kw:"+resource, res.Marked))
	}
}

// recordUpsert counts one imported record; kind is "drive" or "charge" and id its TeslaMate identifier.
func (st *resourceSyncStats) recordUpsert(isInserted bool, err error, kind string, id int) {
	if err != nil {
		st.failed++
		if st.failed <= maxDetailedSyncWarnings {
			st.warnings = append(st.warnings, apierror.NewMessagef("sync.record_failed", "TeslaMate %s #%d not imported: %v", kind, id, err))
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

func (st *resourceSyncStats) finalize(resource string) {
	if st.failed > maxDetailedSyncWarnings {
		st.warnings = append(st.warnings, apierror.NewMessagef("sync.records_failed", "%s: %d items not imported in total", "kw:"+resource, st.failed))
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

// finishResourceSync closes the sync of one resource: it summarizes the results and, when every page was read
// without failure, reconciles the local rows and records the success of the sync.
func (s *SyncService) finishResourceSync(ctx context.Context, st *resourceSyncStats, vehicleID, resource string, completed, fullPass bool) {
	st.finalize(resource)
	if completed && st.failed == 0 {
		s.reconcile(ctx, st, vehicleID, resource, fullPass)
		if err := s.repo.MarkSyncSuccess(ctx, vehicleID, resource, true); err != nil {
			st.warnings = append(st.warnings, apierror.NewMessagef("sync.state_not_saved", "%s: synchronization state not saved (%v)", "kw:"+resource, err))
		}
	}
}
