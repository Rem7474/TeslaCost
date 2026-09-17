package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/teslacost/teslacost/internal/database"
	"github.com/teslacost/teslacost/internal/models"
)

// NotificationService handles evaluation of maintenance reminders and dispatching homelab webhooks.
type NotificationService struct {
	repo       *database.Repository
	httpClient *http.Client
}

// NewNotificationService creates a new NotificationService instance.
func NewNotificationService(repo *database.Repository) *NotificationService {
	return &NotificationService{
		repo: repo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// CheckAndNotify evaluates reminders for a vehicle and sends webhooks if due thresholds are crossed.
func (s *NotificationService) CheckAndNotify(ctx context.Context, vehicle *models.Vehicle, currentOdometer float64) error {
	if s.repo == nil || vehicle == nil {
		return nil
	}

	webhook, err := s.repo.GetVehicleWebhook(ctx, vehicle.ID)
	if err != nil {
		return fmt.Errorf("failed to get vehicle webhook: %w", err)
	}
	if webhook == nil || !webhook.Enabled || strings.TrimSpace(webhook.URL) == "" {
		return nil // No active webhook configured
	}

	reminders, err := s.repo.ListMaintenanceReminders(ctx, vehicle.ID, currentOdometer)
	if err != nil {
		return fmt.Errorf("failed to list reminders: %w", err)
	}

	now := time.Now()
	for _, rem := range reminders {
		if !rem.WebhookEnabled || rem.Status == "OK" {
			continue
		}

		// Anti-spam rule: do not re-notify if notified in the last 7 days AND odometer hasn't changed by >= 500 km
		if rem.LastNotifiedAt != nil {
			timeSinceNotification := now.Sub(*rem.LastNotifiedAt)
			odoDelta := 0.0
			if rem.LastNotifiedOdometer != nil {
				odoDelta = currentOdometer - *rem.LastNotifiedOdometer
				if odoDelta < 0 {
					odoDelta = -odoDelta
				}
			}
			if timeSinceNotification < 7*24*time.Hour && odoDelta < 500 {
				continue
			}
		}

		// Dispatch notification
		if err := s.sendReminderWebhook(ctx, webhook, vehicle.Name, &rem, currentOdometer); err != nil {
			log.Printf("[notification] Failed to dispatch webhook for vehicle %s reminder %s: %v", vehicle.ID, rem.ID, err)
			continue
		}

		// Update last notified
		if err := s.repo.MarkReminderNotified(ctx, rem.ID, now, currentOdometer); err != nil {
			log.Printf("[notification] Failed to mark reminder notified: %v", err)
		}
	}

	return nil
}

// TestWebhook dispatches a test message to verify connectivity and configuration.
func (s *NotificationService) TestWebhook(ctx context.Context, webhook *models.VehicleWebhook, vehicleName string) error {
	testReminder := &models.MaintenanceReminder{
		Title:  "Test de notification",
		Status: "DUE_SOON",
	}
	remKm := 500.0
	testReminder.RemainingKm = &remKm
	remDays := 15
	testReminder.RemainingDays = &remDays

	return s.sendReminderWebhook(ctx, webhook, vehicleName, testReminder, 50000)
}

func (s *NotificationService) sendReminderWebhook(
	ctx context.Context,
	webhook *models.VehicleWebhook,
	vehicleName string,
	rem *models.MaintenanceReminder,
	currentOdo float64,
) error {
	payload, err := formatPayload(webhook.Type, vehicleName, rem, currentOdo)
	if err != nil {
		return err
	}
	return s.postWebhook(ctx, webhook.URL, payload)
}

// NotifySyncCircuitOpen alerts a vehicle's configured webhook that its TeslaMate
// synchronization has failed repeatedly and has been automatically suspended until retryAt.
// Unlike maintenance reminders this is not evaluated on a schedule: callers are expected to
// invoke it exactly once, right when a circuit breaker trips from CLOSED/HALF_OPEN to OPEN.
func (s *NotificationService) NotifySyncCircuitOpen(ctx context.Context, vehicle *models.Vehicle, cause error, retryAt time.Time) error {
	if s.repo == nil || vehicle == nil {
		return nil
	}

	webhook, err := s.repo.GetVehicleWebhook(ctx, vehicle.ID)
	if err != nil {
		return fmt.Errorf("failed to get vehicle webhook: %w", err)
	}
	if webhook == nil || !webhook.Enabled || strings.TrimSpace(webhook.URL) == "" {
		return nil // No active webhook configured
	}

	payload := formatSyncAlertPayload(webhook.Type, vehicle.Name, cause, retryAt)
	return s.postWebhook(ctx, webhook.URL, payload)
}

// postWebhook serializes and POSTs payload to url, shared by all webhook notification kinds.
func (s *NotificationService) postWebhook(ctx context.Context, webhookURL string, payload any) error {
	url := strings.TrimSpace(webhookURL)
	if url == "" {
		return fmt.Errorf("webhook URL is empty")
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TeslaCost-NotificationService/1.0")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute webhook request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook endpoint returned HTTP %d", resp.StatusCode)
	}

	return nil
}

func formatPayload(
	webhookType, vehicleName string,
	rem *models.MaintenanceReminder,
	currentOdo float64,
) (any, error) {
	statusLabel := "À prévoir prochainement"
	statusEmoji := "??"
	color := 16753920 // Amber

	if rem.Status == "OVERDUE" {
		statusLabel = "EN RETARD"
		statusEmoji = "??"
		color = 15158332 // Red
	}

	details := buildDetailsString(rem)

	switch strings.ToUpper(webhookType) {
	case "DISCORD":
		return map[string]any{
			"username":   "TeslaCost",
			"avatar_url": "https://raw.githubusercontent.com/Rem7474/TeslaCost/main/web/public/favicon.svg",
			"embeds": []map[string]any{
				{
					"title":       fmt.Sprintf("%s %s : %s", statusEmoji, statusLabel, rem.Title),
					"description": fmt.Sprintf("**Véhicule :** %s\n**Odomètre :** %.0f km\n\n%s", vehicleName, currentOdo, details),
					"color":       color,
					"footer": map[string]string{
						"text": "TeslaCost • Suivi d'entretien",
					},
					"timestamp": time.Now().Format(time.RFC3339),
				},
			},
		}, nil

	case "TELEGRAM":
		text := fmt.Sprintf(
			"?? *TeslaCost — Rappel d'Entretien*\n\n%s *%s*\nOpération : *%s*\nVéhicule : *%s*\nOdomètre : %.0f km\n%s",
			statusEmoji, statusLabel, rem.Title, vehicleName, currentOdo, details,
		)
		return map[string]any{
			"text":       text,
			"parse_mode": "Markdown",
		}, nil

	case "GOTIFY":
		priority := 5
		if rem.Status == "OVERDUE" {
			priority = 8
		}
		return map[string]any{
			"title":    fmt.Sprintf("TeslaCost : %s (%s)", rem.Title, statusLabel),
			"message":  fmt.Sprintf("Véhicule : %s\nOdomètre : %.0f km\n%s", vehicleName, currentOdo, details),
			"priority": priority,
		}, nil

	default: // GENERIC
		return map[string]any{
			"event":            "maintenance_reminder",
			"status":           rem.Status,
			"title":            rem.Title,
			"vehicle_name":     vehicleName,
			"current_odometer": currentOdo,
			"remaining_km":     rem.RemainingKm,
			"remaining_days":   rem.RemainingDays,
			"timestamp":        time.Now().Format(time.RFC3339),
		}, nil
	}
}

// formatSyncAlertPayload builds a webhook payload announcing that automatic TeslaMate
// synchronization has been suspended for a vehicle after repeated failures.
func formatSyncAlertPayload(webhookType, vehicleName string, cause error, retryAt time.Time) any {
	message := fmt.Sprintf(
		"La synchronisation TeslaMate a échoué plusieurs fois de suite et a été suspendue automatiquement (nouvel essai après %s).\nDernière erreur : %v",
		retryAt.Format("02/01/2006 15:04 MST"), cause,
	)

	switch strings.ToUpper(webhookType) {
	case "DISCORD":
		return map[string]any{
			"username":   "TeslaCost",
			"avatar_url": "https://raw.githubusercontent.com/Rem7474/TeslaCost/main/web/public/favicon.svg",
			"embeds": []map[string]any{
				{
					"title":       fmt.Sprintf("[ALERTE] Synchronisation TeslaMate en échec : %s", vehicleName),
					"description": message,
					"color":       15158332, // Red
					"footer": map[string]string{
						"text": "TeslaCost • Alerte système",
					},
					"timestamp": time.Now().Format(time.RFC3339),
				},
			},
		}

	case "TELEGRAM":
		return map[string]any{
			"text":       fmt.Sprintf("*TeslaCost — Alerte synchronisation*\n\nVéhicule : *%s*\n%s", vehicleName, message),
			"parse_mode": "Markdown",
		}

	case "GOTIFY":
		return map[string]any{
			"title":    fmt.Sprintf("TeslaCost : synchronisation en échec (%s)", vehicleName),
			"message":  message,
			"priority": 8,
		}

	default: // GENERIC
		return map[string]any{
			"event":        "sync_circuit_open",
			"vehicle_name": vehicleName,
			"error":        cause.Error(),
			"retry_at":     retryAt.Format(time.RFC3339),
			"timestamp":    time.Now().Format(time.RFC3339),
		}
	}
}

func buildDetailsString(rem *models.MaintenanceReminder) string {
	var parts []string
	if rem.RemainingKm != nil {
		if *rem.RemainingKm <= 0 {
			parts = append(parts, fmt.Sprintf("Kilométrage : Dépassé de %.0f km", -*rem.RemainingKm))
		} else {
			parts = append(parts, fmt.Sprintf("Kilométrage : Dans %.0f km", *rem.RemainingKm))
		}
	}
	if rem.RemainingDays != nil {
		if *rem.RemainingDays <= 0 {
			parts = append(parts, fmt.Sprintf("Échéance : Dépassée de %d jour(s)", -*rem.RemainingDays))
		} else {
			parts = append(parts, fmt.Sprintf("Échéance : Dans %d jour(s)", *rem.RemainingDays))
		}
	}
	if len(parts) == 0 {
		return "Échéance atteinte"
	}
	return strings.Join(parts, " • ")
}
