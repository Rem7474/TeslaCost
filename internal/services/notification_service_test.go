package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/teslacost/teslacost/internal/models"
)

// fakeNotificationStore is an in-memory notificationStore, letting CheckAndNotify's business
// logic (anti-spam window, webhook dispatch) be tested without a real database.
type fakeNotificationStore struct {
	webhook          *models.VehicleWebhook
	reminders        []models.MaintenanceReminder
	markedNotifiedID string
}

func (f *fakeNotificationStore) GetVehicleWebhook(ctx context.Context, vehicleID string) (*models.VehicleWebhook, error) {
	return f.webhook, nil
}

func (f *fakeNotificationStore) ListMaintenanceReminders(ctx context.Context, vehicleID string, currentOdo float64) ([]models.MaintenanceReminder, error) {
	return f.reminders, nil
}

func (f *fakeNotificationStore) MarkReminderNotified(ctx context.Context, reminderID string, notifiedAt time.Time, notifiedOdo float64) error {
	f.markedNotifiedID = reminderID
	return nil
}

func TestCheckAndNotifyDispatchesDueReminder(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	store := &fakeNotificationStore{
		webhook: &models.VehicleWebhook{URL: server.URL, Type: "GENERIC", Enabled: true},
		reminders: []models.MaintenanceReminder{
			{ID: "rem-1", Title: "Vidange", Status: "OVERDUE", WebhookEnabled: true},
		},
	}
	svc := NewNotificationService(store)

	if err := svc.CheckAndNotify(context.Background(), &models.Vehicle{ID: "v1", Name: "Model 3"}, 50000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Fatalf("expected exactly 1 webhook call, got %d", callCount)
	}
	if store.markedNotifiedID != "rem-1" {
		t.Fatalf("expected reminder rem-1 to be marked notified, got %q", store.markedNotifiedID)
	}
}

func TestCheckAndNotifySkipsRecentlyNotifiedReminder(t *testing.T) {
	var callCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifiedAt := time.Now().Add(-24 * time.Hour) // 1 day ago: inside the 7-day anti-spam window
	notifiedOdo := 49900.0                        // 100 km ago: below the 500 km anti-spam threshold
	store := &fakeNotificationStore{
		webhook: &models.VehicleWebhook{URL: server.URL, Type: "GENERIC", Enabled: true},
		reminders: []models.MaintenanceReminder{
			{
				ID: "rem-1", Title: "Vidange", Status: "OVERDUE", WebhookEnabled: true,
				LastNotifiedAt: &notifiedAt, LastNotifiedOdometer: &notifiedOdo,
			},
		},
	}
	svc := NewNotificationService(store)

	if err := svc.CheckAndNotify(context.Background(), &models.Vehicle{ID: "v1", Name: "Model 3"}, 50000); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callCount != 0 {
		t.Fatalf("expected the anti-spam window to suppress the webhook, got %d calls", callCount)
	}
}

func TestFormatPayloadDiscord(t *testing.T) {
	remKm := 450.0
	rem := &models.MaintenanceReminder{
		Title:       "Permutation des pneus",
		Status:      "DUE_SOON",
		RemainingKm: &remKm,
	}

	payload, err := formatPayload("DISCORD", "Model 3", rem, 42000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, ok := payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", payload)
	}
	embeds, ok := m["embeds"].([]map[string]any)
	if !ok || len(embeds) == 0 {
		t.Fatalf("expected embeds array")
	}
	if embeds[0]["color"] != 16753920 { // Amber for DUE_SOON
		t.Errorf("expected amber color, got %v", embeds[0]["color"])
	}
}

func TestFormatPayloadTelegramAndGotify(t *testing.T) {
	remKm := -120.0
	rem := &models.MaintenanceReminder{
		Title:       "Filtre habitacle",
		Status:      "OVERDUE",
		RemainingKm: &remKm,
	}

	// Telegram
	tgPayload, err := formatPayload("TELEGRAM", "Model Y", rem, 55000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tgMap := tgPayload.(map[string]any)
	if tgMap["parse_mode"] != "Markdown" {
		t.Errorf("expected Markdown parse mode")
	}

	// Gotify
	gotifyPayload, err := formatPayload("GOTIFY", "Model Y", rem, 55000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotifyMap := gotifyPayload.(map[string]any)
	if gotifyMap["priority"] != 8 {
		t.Errorf("expected priority 8 for OVERDUE, got %v", gotifyMap["priority"])
	}
}

func TestFormatSyncAlertPayload(t *testing.T) {
	cause := context.DeadlineExceeded
	retryAt := time.Date(2026, 9, 17, 18, 0, 0, 0, time.UTC)

	discord := formatSyncAlertPayload("DISCORD", "Model 3", cause, retryAt).(map[string]any)
	embeds, ok := discord["embeds"].([]map[string]any)
	if !ok || len(embeds) == 0 {
		t.Fatalf("expected embeds array")
	}
	if embeds[0]["color"] != 15158332 {
		t.Errorf("expected red color for a sync failure alert, got %v", embeds[0]["color"])
	}

	generic := formatSyncAlertPayload("GENERIC", "Model 3", cause, retryAt).(map[string]any)
	if generic["event"] != "sync_circuit_open" {
		t.Errorf("expected sync_circuit_open event, got %v", generic["event"])
	}
	if generic["vehicle_name"] != "Model 3" {
		t.Errorf("expected vehicle_name to be set, got %v", generic["vehicle_name"])
	}
}

func TestNotifySyncCircuitOpenWithoutRepoIsNoop(t *testing.T) {
	svc := NewNotificationService(nil)
	err := svc.NotifySyncCircuitOpen(context.Background(), &models.Vehicle{ID: "v1", Name: "Model 3"}, errors.New("boom"), time.Now())
	if err != nil {
		t.Fatalf("expected no error without a repo, got %v", err)
	}
}

func TestSendReminderWebhook(t *testing.T) {
	serverCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewNotificationService(nil)
	webhook := &models.VehicleWebhook{
		URL:     server.URL,
		Type:    "DISCORD",
		Enabled: true,
	}

	err := svc.TestWebhook(context.Background(), webhook, "Tesla Test")
	if err != nil {
		t.Fatalf("unexpected test webhook error: %v", err)
	}
	if !serverCalled {
		t.Errorf("expected webhook endpoint to be called")
	}
}

func TestComputeStatus(t *testing.T) {
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	intKm := 10000
	lastOdo := 30000.0
	rem := &models.MaintenanceReminder{
		IntervalKm:          &intKm,
		LastServiceOdometer: &lastOdo,
		LeadKm:              1000,
	}

	// 1. Odo = 35000 -> OK (5000 km left)
	rem.ComputeStatus(35000, now)
	if rem.Status != "OK" || *rem.RemainingKm != 5000 {
		t.Errorf("expected OK with 5000 km left, got %s, %v", rem.Status, rem.RemainingKm)
	}

	// 2. Odo = 39200 -> DUE_SOON (800 km left <= 1000 leadKm)
	rem.ComputeStatus(39200, now)
	if rem.Status != "DUE_SOON" || *rem.RemainingKm != 800 {
		t.Errorf("expected DUE_SOON with 800 km left, got %s, %v", rem.Status, rem.RemainingKm)
	}

	// 3. Odo = 40100 -> OVERDUE (-100 km)
	rem.ComputeStatus(40100, now)
	if rem.Status != "OVERDUE" || *rem.RemainingKm != -100 {
		t.Errorf("expected OVERDUE with -100 km left, got %s, %v", rem.Status, rem.RemainingKm)
	}
}
