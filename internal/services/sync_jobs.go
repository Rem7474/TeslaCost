package services

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

// Sync job statuses.
const (
	SyncJobRunning   = "RUNNING"
	SyncJobSucceeded = "SUCCEEDED"
	SyncJobFailed    = "FAILED"
)

// syncJobTimeout bounds a single synchronization, long enough for a first full history import.
const syncJobTimeout = 15 * time.Minute

// SyncJob tracks the last synchronization of a vehicle.
type SyncJob struct {
	VehicleID  string      `json:"vehicle_id"`
	Status     string      `json:"status"`
	Trigger    string      `json:"trigger"` // MANUAL | SCHEDULED
	StartedAt  time.Time   `json:"started_at"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
	Result     *SyncResult `json:"result,omitempty"`
	Error      string      `json:"error,omitempty"`
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
		ctx, cancel := context.WithTimeout(context.Background(), syncJobTimeout)
		defer cancel()
		res, err := s.SyncVehicle(ctx, &v)
		s.jobs.finish(v.ID, res, err)
	}()
	return job, true
}

// GetSyncJob returns the last synchronization job of a vehicle since the server started, if any.
func (s *SyncService) GetSyncJob(vehicleID string) *SyncJob {
	return s.jobs.get(vehicleID)
}

// runScheduledSync synchronizes a vehicle from the background worker unless a manual sync is running.
func (s *SyncService) runScheduledSync(ctx context.Context, v models.Vehicle) {
	if _, started := s.jobs.begin(v.ID, "SCHEDULED"); !started {
		return
	}
	vCtx, cancel := context.WithTimeout(ctx, syncJobTimeout)
	res, err := s.SyncVehicle(vCtx, &v)
	cancel()
	s.jobs.finish(v.ID, res, err)

	if err != nil {
		log.Printf("[auto-sync] Vehicle %s (%s): sync warning/error: %v", v.Name, v.ID, err)
	} else if res != nil && (res.DrivesAdded > 0 || res.ChargesAdded > 0) {
		log.Printf("[auto-sync] Vehicle %s (%s): +%d new drives, +%d new charges (odometer: %.0f km)",
			v.Name, v.ID, res.DrivesAdded, res.ChargesAdded, res.CurrentOdometer)
	}
}
