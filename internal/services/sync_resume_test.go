package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

type fakeSyncStore struct {
	drives       map[int]*models.Drive
	charges      map[int]*models.ChargeLog
	fullImported map[string]bool
}

func newFakeSyncStore() *fakeSyncStore {
	return &fakeSyncStore{drives: map[int]*models.Drive{}, charges: map[int]*models.ChargeLog{}, fullImported: map[string]bool{}}
}

func (f *fakeSyncStore) UpdateVehicleOdometer(context.Context, string, float64) error { return nil }

func (f *fakeSyncStore) GetLatestTeslaMateDriveStartTime(context.Context, string) (*time.Time, error) {
	var latest *time.Time
	for _, d := range f.drives {
		if latest == nil || d.StartTime.After(*latest) {
			t := d.StartTime
			latest = &t
		}
	}
	return latest, nil
}

func (f *fakeSyncStore) GetLatestTeslaMateChargeDate(context.Context, string) (*time.Time, error) {
	var latest *time.Time
	for _, c := range f.charges {
		if latest == nil || c.Date.After(*latest) {
			t := c.Date
			latest = &t
		}
	}
	return latest, nil
}

func (f *fakeSyncStore) UpsertTeslaMateDrive(_ context.Context, d *models.Drive) (bool, error) {
	_, exists := f.drives[*d.TeslaMateDriveID]
	f.drives[*d.TeslaMateDriveID] = d
	return !exists, nil
}

func (f *fakeSyncStore) UpsertTeslaMateCharge(_ context.Context, c *models.ChargeLog) (bool, error) {
	_, exists := f.charges[*c.TeslaMateChargeID]
	f.charges[*c.TeslaMateChargeID] = c
	return !exists, nil
}

func (f *fakeSyncStore) IsFullImportCompleted(_ context.Context, _ string, resource string) (bool, error) {
	return f.fullImported[resource], nil
}

func (f *fakeSyncStore) MarkSyncSuccess(_ context.Context, _ string, resource string, fullImport bool) error {
	if fullImport {
		f.fullImported[resource] = true
	}
	return nil
}

func (f *fakeSyncStore) ListAllVehiclesWithTeslaMate(context.Context) ([]models.Vehicle, error) {
	return nil, nil
}

// fakeTeslaMate serves `total` drives and charges (newest first, one per day) and can fail a given page.
type fakeTeslaMate struct {
	total     int
	failPage  int
	chargeFee func(id int) *float64
}

func (m *fakeTeslaMate) handler(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	show, _ := strconv.Atoi(r.URL.Query().Get("show"))
	base := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)
	w.Header().Set("Content-Type", "application/json")

	if r.URL.Path == "/api/v1/cars/1/status" {
		_, _ = w.Write([]byte(`{"data":{"status":{"odometer":1000},"units":{"unit_of_length":"km"}}}`))
		return
	}
	if page == m.failPage {
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	var items []map[string]any
	for i := (page - 1) * show; i < page*show && i < m.total; i++ {
		id := m.total - i // newest first
		date := base.AddDate(0, 0, id).Format(time.RFC3339)
		if r.URL.Path == "/api/v1/cars/1/drives" {
			items = append(items, map[string]any{"drive_id": id, "start_date": date, "end_date": date,
				"odometer_details": map[string]any{"odometer_distance": 10}})
		} else {
			item := map[string]any{"charge_id": id, "start_date": date, "end_date": date, "charge_energy_added": 20}
			if m.chargeFee != nil {
				item["cost"] = m.chargeFee(id)
			}
			items = append(items, item)
		}
	}
	key := "drives"
	if r.URL.Path == "/api/v1/cars/1/charges" {
		key = "charges"
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{key: items, "units": map[string]any{"unit_of_length": "km"}}})
}

func newTestSync(t *testing.T, tm *fakeTeslaMate) (*SyncService, *fakeSyncStore, *models.Vehicle) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(tm.handler))
	t.Cleanup(server.Close)
	store := newFakeSyncStore()
	url := server.URL
	return &SyncService{repo: store}, store, &models.Vehicle{ID: "v1", TeslaMateAPIURL: &url, TeslaMateAuthType: models.AuthModeNone}
}

func TestSyncResumesInterruptedFullImport(t *testing.T) {
	tm := &fakeTeslaMate{total: 170, failPage: 3} // 4 pages of 50; page 3 fails
	svc, store, v := newTestSync(t, tm)

	res, err := svc.SyncVehicle(context.Background(), v)
	if err != nil {
		t.Fatalf("first sync failed: %v", err)
	}
	if len(store.drives) != 100 || len(res.Warnings) == 0 {
		t.Fatalf("expected 100 drives and warnings after interrupted import, got %d drives, warnings=%v", len(store.drives), res.Warnings)
	}
	if store.fullImported["drives"] {
		t.Fatal("an interrupted import must not be marked as complete")
	}

	tm.failPage = 0
	if _, err := svc.SyncVehicle(context.Background(), v); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}
	if len(store.drives) != 170 || len(store.charges) != 170 {
		t.Fatalf("expected full history (170 drives / 170 charges) after resume, got %d / %d", len(store.drives), len(store.charges))
	}
	if !store.fullImported["drives"] || !store.fullImported["charges"] {
		t.Fatal("expected full import to be marked complete")
	}
}

func TestSyncRefreshesRecentCostsAndKeepsUnknownCostNil(t *testing.T) {
	tm := &fakeTeslaMate{total: 120}
	svc, store, v := newTestSync(t, tm)

	if _, err := svc.SyncVehicle(context.Background(), v); err != nil {
		t.Fatalf("initial sync failed: %v", err)
	}
	if c := store.charges[120]; c.Cost != nil {
		t.Fatalf("a charge without tariff must keep a nil cost, got %v", *c.Cost)
	}

	// Costs configured afterwards in TeslaMate for every charge.
	tm.chargeFee = func(id int) *float64 { v := float64(id); return &v }
	if _, err := svc.SyncVehicle(context.Background(), v); err != nil {
		t.Fatalf("incremental sync failed: %v", err)
	}

	for _, id := range []int{120, 100, 91} { // within the 30-day overlap window of the newest charge
		if c := store.charges[id]; c.Cost == nil {
			t.Errorf("charge %d: expected cost refreshed inside the overlap window", id)
		}
	}
	if c := store.charges[20]; c.Cost != nil {
		t.Errorf("charge 20: incremental sync should stop before old history, got cost %v", *c.Cost)
	}
}
