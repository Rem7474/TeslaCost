package services

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teslacost/teslacost/internal/crypto"
	"github.com/teslacost/teslacost/internal/models"
)

func TestFormatTeslaMateError(t *testing.T) {
	// Test Docker localhost hint
	errRefused := errors.New("dial tcp 127.0.0.1:8087: connect: connection refused")
	formatted := formatTeslaMateError(errRefused, "http://localhost:8087")
	if !strings.Contains(formatted.Error(), "host.docker.internal") {
		t.Errorf("Expected host.docker.internal in error message, got %s", formatted.Error())
	}

	// Test timeout hint
	errTimeout := errors.New("context deadline exceeded (Client.Timeout exceeded while awaiting headers)")
	formattedTimeout := formatTeslaMateError(errTimeout, "http://192.168.1.50:8087")
	if !strings.Contains(formattedTimeout.Error(), "Délai d'attente dépassé") {
		t.Errorf("Expected timeout hint in error message, got %s", formattedTimeout.Error())
	}
}

func TestTestConnectionRaw(t *testing.T) {
	// Mock TeslaMate API server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/cars/1/status" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"data": {
					"status": {
						"state": "asleep",
						"battery_level": 80,
						"usable_battery_level": 79,
						"odometer": 15420.5
					},
					"units": {
						"unit_of_length": "km",
						"unit_of_temperature": "C"
					}
				}
			}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	enc, err := crypto.NewEncryptor("test-secret-key-12345")
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v", err)
	}

	svc := NewSyncService(nil, enc)

	// Test success
	status, err := svc.TestConnectionRaw(context.Background(), ts.URL, models.AuthModeNone, "", "", "", 1)
	if err != nil {
		t.Fatalf("Expected connection test to succeed, got %v", err)
	}
	if status.State != "asleep" {
		t.Errorf("Expected state asleep, got %s", status.State)
	}
	if status.Odometer != 15420.5 {
		t.Errorf("Expected odometer 15420.5, got %f", status.Odometer)
	}

	// Test empty URL
	_, errEmpty := svc.TestConnectionRaw(context.Background(), "", models.AuthModeNone, "", "", "", 1)
	if errEmpty == nil {
		t.Errorf("Expected error with empty URL")
	}
}
