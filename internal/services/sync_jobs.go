package services

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/teslacost/teslacost/internal/apierror"
	"github.com/teslacost/teslacost/internal/models"
)

// recoverPanic logs and swallows a panic recovered from a background goroutine so that
// an unexpected error in a single background job (e.g. a malformed TeslaMate response)
// cannot crash the whole server process.
func recoverPanic(tag string) {
	if r := recover(); r != nil {
		slog.Error("recovered panic in background goroutine", "component", tag, "panic", r)
	}
}

// recordSyncFailure records a sync failure on the vehicle's circuit breaker and, only on the
// failure that trips the breaker from CLOSED/HALF_OPEN to OPEN, dispatches a best-effort webhook
// alert so a repeatedly failing sync doesn't go unnoticed until someone opens the app.
func (s *SyncService) recordSyncFailure(v models.Vehicle, cb *CircuitBreaker, syncErr error) {
	wasOpen := cb.State() == CircuitOpen
	cb.RecordFailure(syncErr)

	if wasOpen || cb.State() != CircuitOpen || s.notifications == nil {
		return
	}
	retryAt := cb.NextRetry()
	go func(veh models.Vehicle, cause error, retry time.Time) {
		defer recoverPanic("sync.circuitBreakerAlert")
		alertCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.notifications.NotifySyncCircuitOpen(alertCtx, &veh, cause, retry); err != nil {
			slog.Error("failed to alert on circuit breaker open", "component", "notification", "vehicle_id", veh.ID, "error", err)
		}
	}(v, syncErr, retryAt)
}

// Sync job statuses.
const (
	SyncJobRunning   = "RUNNING"
	SyncJobSucceeded = "SUCCEEDED"
	SyncJobFailed    = "FAILED"
)

// Sync job timeouts.
const (
	// manualSyncTimeout bounds a user-triggered synchronization, long enough for a first full history import.
	manualSyncTimeout = 15 * time.Minute
	// scheduledSyncTimeout bounds a periodic background synchronization to avoid hanging worker loops.
	scheduledSyncTimeout = 3 * time.Minute
)

// SyncJob tracks the last synchronization of a vehicle.
type SyncJob struct {
	VehicleID  string      `json:"vehicle_id"`
	Status     string      `json:"status"`
	Trigger    string      `json:"trigger"` // MANUAL | SCHEDULED
	StartedAt  time.Time   `json:"started_at"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
	Result     *SyncResult `json:"result,omitempty"`
	Error      string      `json:"error,omitempty"`
	// ErrorCode and ErrorParams let the front end translate Error.
	ErrorCode   string         `json:"error_code,omitempty"`
	ErrorParams map[string]any `json:"error_params,omitempty"`
}

// syncJobs holds in-memory job states; a vehicle never has two synchronizations running at once.
type syncJobs struct {
	mu   sync.Mutex
	jobs map[string]*SyncJob
}

// begin registers a running job, or returns the job already running for the vehicle.
func (j *syncJobs) begin(vehicleID, trigger string) (*SyncJob, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.jobs == nil {
		j.jobs = make(map[string]*SyncJob)
	}
	if existing, ok := j.jobs[vehicleID]; ok && existing.Status == SyncJobRunning {
		copy := *existing
		return &copy, false
	}
	job := &SyncJob{VehicleID: vehicleID, Status: SyncJobRunning, Trigger: trigger, StartedAt: time.Now().UTC()}
	j.jobs[vehicleID] = job
	copy := *job
	return &copy, true
}

func (j *syncJobs) finish(vehicleID string, res *SyncResult, err error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	job, ok := j.jobs[vehicleID]
	if !ok {
		return
	}
	now := time.Now().UTC()
	job.FinishedAt = &now
	job.Result = res
	if err != nil {
		job.Status = SyncJobFailed
		job.Error = err.Error()
		if apiErr, ok := apierror.As(err); ok {
			job.ErrorCode, job.ErrorParams = apiErr.Code, apiErr.Params
		}
	} else {
		job.Status = SyncJobSucceeded
	}
}

func (j *syncJobs) get(vehicleID string) *SyncJob {
	j.mu.Lock()
	defer j.mu.Unlock()
	job, ok := j.jobs[vehicleID]
	if !ok {
		return nil
	}
	copy := *job
	return &copy
}

// StartSync launches a synchronization in the background and returns its job immediately.
// started is false when a synchronization of this vehicle is already running.
func (s *SyncService) StartSync(v models.Vehicle) (job *SyncJob, started bool) {
	job, started = s.jobs.begin(v.ID, "MANUAL")
	if !started {
		return job, false
	}
	go func() {
		defer recoverPanic("sync.StartSync")
		ctx, cancel := context.WithTimeout(context.Background(), manualSyncTimeout)
		defer cancel()
		res, err := s.SyncVehicle(ctx, &v)
		cb := s.getCircuitBreaker(v.ID)
		if err != nil {
			s.recordSyncFailure(v, cb, err)
		} else {
			cb.RecordSuccess()
		}
		s.jobs.finish(v.ID, res, err)
	}()
	return job, true
}

// GetSyncJob returns the last synchronization job of a vehicle since the server started, if any.
func (s *SyncService) GetSyncJob(vehicleID string) *SyncJob {
	return s.jobs.get(vehicleID)
}

// runScheduledSync synchronizes a vehicle from the background worker unless a manual sync is running
// or the vehicle's circuit breaker is currently open.
func (s *SyncService) runScheduledSync(ctx context.Context, v models.Vehicle) {
	cb := s.getCircuitBreaker(v.ID)
	if err := cb.CanExecute(); err != nil {
		slog.Info("skipping scheduled sync", "component", "auto-sync", "vehicle_name", v.Name, "vehicle_id", v.ID, "reason", err)
		return
	}

	if _, started := s.jobs.begin(v.ID, "SCHEDULED"); !started {
		return
	}
	vCtx, cancel := context.WithTimeout(ctx, scheduledSyncTimeout)
	res, err := s.SyncVehicle(vCtx, &v)
	cancel()

	if err != nil {
		s.recordSyncFailure(v, cb, err)
		slog.Warn("scheduled sync failed", "component", "auto-sync", "vehicle_name", v.Name, "vehicle_id", v.ID, "error", err, "circuit_breaker", cb.State())
	} else {
		cb.RecordSuccess()
		if res != nil && (res.DrivesAdded > 0 || res.ChargesAdded > 0) {
			slog.Info("scheduled sync completed", "component", "auto-sync", "vehicle_name", v.Name, "vehicle_id", v.ID,
				"drives_added", res.DrivesAdded, "charges_added", res.ChargesAdded, "odometer_km", res.CurrentOdometer)
		}
	}

	s.jobs.finish(v.ID, res, err)
}
