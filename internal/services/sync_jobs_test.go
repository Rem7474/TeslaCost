package services

import (
	"errors"
	"testing"
	"time"
)

func TestSyncJobsSingleRunPerVehicle(t *testing.T) {
	var jobs syncJobs
	first, started := jobs.begin("v1", "MANUAL")
	if !started || first.Status != SyncJobRunning {
		t.Fatalf("expected a running job, got %+v", first)
	}
	if _, started := jobs.begin("v1", "SCHEDULED"); started {
		t.Fatal("a second synchronization of the same vehicle must not start")
	}
	if _, started := jobs.begin("v2", "SCHEDULED"); !started {
		t.Fatal("another vehicle can synchronize concurrently")
	}

	jobs.finish("v1", nil, errors.New("boom"))
	if job := jobs.get("v1"); job.Status != SyncJobFailed || job.Error != "boom" || job.FinishedAt == nil {
		t.Fatalf("unexpected finished job: %+v", job)
	}
	if _, started := jobs.begin("v1", "MANUAL"); !started {
		t.Fatal("a new synchronization can start once the previous one finished")
	}
}

func TestStartSyncRunsInBackground(t *testing.T) {
	tm := &fakeTeslaMate{total: 60}
	svc, store, v := newTestSync(t, tm)

	job, started := svc.StartSync(*v)
	if !started || job.Status != SyncJobRunning {
		t.Fatalf("expected the job to start, got %+v", job)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if j := svc.GetSyncJob(v.ID); j.Status != SyncJobRunning {
			if j.Status != SyncJobSucceeded || j.Result == nil || j.Result.DrivesAdded != 60 || len(store.drives) != 60 {
				t.Fatalf("unexpected job result: %+v", j)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("background synchronization did not finish")
}
